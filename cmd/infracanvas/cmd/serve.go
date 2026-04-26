package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"infracanvas/pkg/agent"
	"infracanvas/pkg/server"
	"infracanvas/pkg/tunnel"
	"infracanvas/pkg/webui"
)

const (
	defaultPort       = 7777
	portFallbackTries = 14 // 7777..7790
)

var (
	servePort     int
	servePrivate  bool
	serveNoTunnel bool
	serveUIToken  string
	serveScope    []string
	serveRefresh  int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the InfraCanvas dashboard and agent on this machine",
	Long: `serve runs the dashboard, relay, and discovery agent in one process.

By default it opens a free Cloudflare quick-tunnel and prints a public URL you
can paste into any browser — no firewall change needed:

    infracanvas serve

If you'd rather expose the dashboard directly (you've opened the port in your
cloud security group, or you're on a private network), pass --no-tunnel:

    infracanvas serve --no-tunnel             # bind 0.0.0.0:7777
    infracanvas serve --no-tunnel --private   # bind 127.0.0.1:7777 (SSH tunnel)

Environment variables:
  INFRACANVAS_UI_TOKEN  Auth token (default: random per run)
  INFRACANVAS_SCOPE     Discovery scopes (default: host,docker,kubernetes)`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&servePort, "port", defaultPort, "Local port (auto-falls-back if taken)")
	serveCmd.Flags().BoolVar(&servePrivate, "private", false, "With --no-tunnel: bind 127.0.0.1 instead of 0.0.0.0")
	serveCmd.Flags().BoolVar(&serveNoTunnel, "no-tunnel", false, "Disable Cloudflare tunnel; bind the port directly")
	serveCmd.Flags().StringVar(&serveUIToken, "token", "", "Override the UI auth token")
	serveCmd.Flags().StringSliceVar(&serveScope, "discover", []string{"host", "docker", "kubernetes"}, "Discovery scopes")
	serveCmd.Flags().IntVar(&serveRefresh, "refresh", 30, "Seconds between discovery refreshes")
}

func runServe(cmd *cobra.Command, args []string) error {
	useTunnel := !serveNoTunnel
	host := "0.0.0.0"
	if useTunnel || servePrivate {
		// Tunnel mode: cloudflared connects on loopback, so the dashboard
		// never needs to be exposed on a public interface.
		host = "127.0.0.1"
	}

	token := resolveToken()
	uiFS, err := webui.FS()
	if err != nil {
		return fmt.Errorf("load embedded UI: %w", err)
	}
	srv := server.NewLocal(token)
	srv.MountUI(uiFS)

	listener, chosenPort, err := bindWithFallback(host, servePort, !cmd.Flags().Changed("port"))
	if err != nil {
		return err
	}

	httpSrv := &http.Server{Handler: srv.Handler()}
	serverErrCh := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Bring up the tunnel if requested.
	var tnl *tunnel.Tunnel
	if useTunnel {
		log.Println("Starting Cloudflare quick-tunnel...")
		tnl, err = tunnel.Start(ctx, fmt.Sprintf("http://127.0.0.1:%d", chosenPort))
		if err != nil {
			cancel()
			_ = httpSrv.Close()
			return fmt.Errorf("start tunnel: %w (try --no-tunnel)", err)
		}
		defer tnl.Stop()
	}

	publicIP := ""
	if !useTunnel && !servePrivate {
		publicIP = detectPublicIP()
	}
	tunnelURL := ""
	if tnl != nil {
		tunnelURL = tnl.URL
	}
	printServeBanner(host, chosenPort, token, publicIP, tunnelURL)

	go func() {
		select {
		case <-sigCh:
			log.Println("Shutting down...")
			cancel()
		case err := <-serverErrCh:
			log.Printf("HTTP server error: %v", err)
			cancel()
		case <-ctx.Done():
		}
		shutdownCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
		defer sc()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	scopes := serveScope
	if envScope := os.Getenv("INFRACANVAS_SCOPE"); envScope != "" {
		scopes = splitComma(envScope)
	}
	cfg := agent.DefaultWSConfig()
	cfg.BackendURL = fmt.Sprintf("ws://127.0.0.1:%d", chosenPort)
	cfg.Scope = scopes
	cfg.RefreshSeconds = serveRefresh
	cfg.EnableRedaction = true
	cfg.QuietPairBanner = true

	for {
		if ctx.Err() != nil {
			return nil
		}
		ag, err := agent.NewWSAgent(cfg)
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}
		if err := ag.Run(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("Agent run ended: %v — restarting in 5s", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func resolveToken() string {
	if serveUIToken != "" {
		return serveUIToken
	}
	if t := os.Getenv("INFRACANVAS_UI_TOKEN"); t != "" {
		return t
	}
	return randomToken(12)
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "infracanvas"
	}
	return hex.EncodeToString(b)
}

// bindWithFallback opens a TCP listener. If the requested port is in use AND
// it was the default (user didn't pass --port), it tries the next few ports.
// With an explicit --port, it fails immediately.
func bindWithFallback(host string, port int, allowFallback bool) (net.Listener, int, error) {
	first := port
	tries := 1
	if allowFallback {
		tries = portFallbackTries
	}
	var lastErr error
	for i := 0; i < tries; i++ {
		p := first + i
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p))
		if err == nil {
			return l, p, nil
		}
		lastErr = err
		if !isAddrInUse(err) {
			return nil, 0, fmt.Errorf("listen on %s:%d: %w", host, p, err)
		}
	}
	if allowFallback {
		return nil, 0, fmt.Errorf("ports %d..%d are all in use — try `--port <free-port>`", first, first+tries-1)
	}
	return nil, 0, fmt.Errorf("port %d is already in use — try `--port <free-port>`: %w", first, lastErr)
}

func isAddrInUse(err error) bool {
	return err != nil && strings.Contains(err.Error(), "address already in use")
}

// detectPublicIP tries cloud metadata endpoints, then a public echo service.
// Returns "" if none answer. Each request has a tight timeout so an offline
// or private VM doesn't block startup.
func detectPublicIP() string {
	for _, fn := range []func() string{azurePublicIP, awsPublicIP, gcpPublicIP, echoServicePublicIP} {
		if ip := fn(); ip != "" {
			return ip
		}
	}
	return ""
}

func httpGetWithTimeout(req *http.Request, timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return ""
	}
	out := strings.TrimSpace(string(body))
	if net.ParseIP(out) == nil {
		return ""
	}
	return out
}

func azurePublicIP() string {
	req, _ := http.NewRequest("GET",
		"http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2021-02-01&format=text",
		nil)
	req.Header.Set("Metadata", "true")
	return httpGetWithTimeout(req, 800*time.Millisecond)
}

func awsPublicIP() string {
	tokenReq, _ := http.NewRequest("PUT", "http://169.254.169.254/latest/api/token", nil)
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	client := &http.Client{Timeout: 600 * time.Millisecond}
	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return ""
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusOK {
		return ""
	}
	tokenBytes, _ := io.ReadAll(io.LimitReader(tokenResp.Body, 256))
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return ""
	}
	req, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/public-ipv4", nil)
	req.Header.Set("X-aws-ec2-metadata-token", token)
	return httpGetWithTimeout(req, 800*time.Millisecond)
}

func gcpPublicIP() string {
	req, _ := http.NewRequest("GET",
		"http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip",
		nil)
	req.Header.Set("Metadata-Flavor", "Google")
	return httpGetWithTimeout(req, 800*time.Millisecond)
}

func echoServicePublicIP() string {
	req, _ := http.NewRequest("GET", "https://api.ipify.org", nil)
	return httpGetWithTimeout(req, 1500*time.Millisecond)
}

func detectInternalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if a, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return ""
}

func printServeBanner(host string, port int, token, publicIP, tunnelURL string) {
	bar := "════════════════════════════════════════════════════════════"
	fmt.Println()
	fmt.Println(bar)
	fmt.Println("  InfraCanvas is running")
	fmt.Println(bar)
	fmt.Println()

	switch {
	case tunnelURL != "":
		// Tunnel mode — the public URL is whatever Cloudflare just gave us.
		fmt.Printf("  Open in your browser:\n")
		fmt.Printf("    \033[1;36m%s/?token=%s\033[0m\n", tunnelURL, token)
		fmt.Println()
		fmt.Println("  This URL works from anywhere — Cloudflare quick-tunnels are free")
		fmt.Println("  and need no firewall changes. The URL is ephemeral; restart the")
		fmt.Println("  service to get a new one. Pass --no-tunnel for a stable URL on")
		fmt.Println("  your own port (requires opening it in your cloud security group).")

	case servePrivate:
		fmt.Printf("  Bound to 127.0.0.1:%d — only this machine can reach it.\n", port)
		fmt.Println()
		fmt.Printf("  Open in this machine's browser:\n")
		fmt.Printf("    http://localhost:%d/?token=%s\n", port, token)
		if internal := detectInternalIP(); internal != "" {
			fmt.Println()
			fmt.Println("  To browse from your laptop, open an SSH tunnel:")
			fmt.Printf("    ssh -L %d:localhost:%d <user>@%s\n", port, port, internal)
			fmt.Printf("    Then open: http://localhost:%d/?token=%s\n", port, token)
		}

	case publicIP != "":
		fmt.Printf("  Open in your browser:\n")
		fmt.Printf("    \033[1;36mhttp://%s:%d/?token=%s\033[0m\n", publicIP, port, token)
		fmt.Println()
		fmt.Printf("  This URL only works if inbound TCP %d is allowed in your\n", port)
		fmt.Println("  cloud security group. Drop --no-tunnel to use Cloudflare's")
		fmt.Println("  free tunnel instead — no firewall changes needed.")

	default:
		internal := detectInternalIP()
		if internal == "" {
			internal = "<this-host>"
		}
		fmt.Printf("  Bound to 0.0.0.0:%d — no public IP detected.\n", port)
		fmt.Println()
		fmt.Printf("  From the same network: http://%s:%d/?token=%s\n", internal, port, token)
		fmt.Println()
		fmt.Println("  From any other network, drop --no-tunnel to use Cloudflare's")
		fmt.Println("  free tunnel — works from a private subnet, on-prem, or behind NAT.")
	}

	fmt.Println()
	fmt.Printf("  Auth token: %s\n", token)
	fmt.Println(bar)
	fmt.Println()
}
