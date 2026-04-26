// Package tunnel manages a Cloudflare quick-tunnel child process that exposes
// the local InfraCanvas dashboard at a public https://*.trycloudflare.com URL
// without requiring any inbound firewall rule.
//
// The cloudflared binary is downloaded on first use into the user cache dir
// (or /tmp/infracanvas as fallback). On macOS we expect cloudflared on PATH
// (e.g. `brew install cloudflared`) since darwin releases ship as tarballs.
package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

// CloudflaredVersion pins the cloudflared release we download. Bump deliberately;
// trycloudflare.com behavior changes occasionally between minor releases.
const CloudflaredVersion = "2024.10.1"

var trycloudflareRE = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// Tunnel is a running cloudflared quick-tunnel.
type Tunnel struct {
	URL string

	cancel  context.CancelFunc
	cmd     *exec.Cmd
	waitErr chan error
}

// Start launches cloudflared, waits for it to publish a public URL, and returns
// once the URL is known. The tunnel keeps running until ctx is cancelled or
// Stop is called.
//
// localURL is the address cloudflared will forward to (e.g. "http://127.0.0.1:7777").
func Start(ctx context.Context, localURL string) (*Tunnel, error) {
	bin, err := ensureCloudflared()
	if err != nil {
		return nil, fmt.Errorf("cloudflared: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, bin,
		"tunnel",
		"--url", localURL,
		"--no-autoupdate",
		"--metrics", "127.0.0.1:0",
	)
	cmd.Stdout = io.Discard
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	urlCh := make(chan string, 1)
	go scanForURL(stderr, urlCh)

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	select {
	case url := <-urlCh:
		return &Tunnel{URL: url, cmd: cmd, cancel: cancel, waitErr: waitErr}, nil
	case err := <-waitErr:
		cancel()
		return nil, fmt.Errorf("cloudflared exited before publishing URL: %w", err)
	case <-time.After(45 * time.Second):
		cancel()
		return nil, fmt.Errorf("cloudflared did not publish a URL within 45s")
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

// Stop terminates the tunnel.
func (t *Tunnel) Stop() {
	if t == nil {
		return
	}
	t.cancel()
	select {
	case <-t.waitErr:
	case <-time.After(5 * time.Second):
		if t.cmd != nil && t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}
	}
}

// Wait blocks until the tunnel exits, returning the exit error if any.
func (t *Tunnel) Wait() error {
	if t == nil {
		return nil
	}
	return <-t.waitErr
}

func scanForURL(r io.Reader, ch chan<- string) {
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	var once sync.Once
	for scan.Scan() {
		line := scan.Text()
		if m := trycloudflareRE.FindString(line); m != "" {
			once.Do(func() { ch <- m })
		}
	}
}

// ensureCloudflared returns the path to a usable cloudflared binary.
// On Linux it's downloaded into the user cache dir if missing; on darwin it
// must already be on PATH (the macOS release is a tarball, not a single binary).
func ensureCloudflared() (string, error) {
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}
	cacheDir := cacheDir()
	binPath := filepath.Join(cacheDir, "cloudflared")
	if info, err := os.Stat(binPath); err == nil && info.Mode()&0o111 != 0 {
		return binPath, nil
	}

	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("cloudflared not on PATH; install it (`brew install cloudflared` on macOS) and retry")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	arch := runtime.GOARCH
	url := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-linux-%s",
		CloudflaredVersion, arch)
	fmt.Fprintf(os.Stderr, "[tunnel] Downloading cloudflared %s for linux/%s (~30 MB, one-time)...\n",
		CloudflaredVersion, arch)

	if err := download(url, binPath); err != nil {
		_ = os.Remove(binPath)
		return "", fmt.Errorf("download cloudflared: %w", err)
	}
	return binPath, nil
}

func cacheDir() string {
	if base, err := os.UserCacheDir(); err == nil {
		return filepath.Join(base, "infracanvas")
	}
	return "/tmp/infracanvas"
}

func download(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}
