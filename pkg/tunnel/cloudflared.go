// Package tunnel manages a Cloudflare quick-tunnel child process that exposes
// the local InfraCanvas dashboard at a public https://*.trycloudflare.com URL
// without requiring any inbound firewall rule.
//
// The cloudflared binary is downloaded on first use into the user cache dir
// (or /tmp/infracanvas as fallback). On macOS we expect cloudflared on PATH
// (e.g. `brew install cloudflared`) since darwin releases ship as tarballs.
//
// Quick-tunnels are inherently fragile: each cloudflared run gets a fresh
// random hostname, the Cloudflare edge can drop the session, and the binary
// itself can crash. Start runs a watchdog that keeps respawning cloudflared
// and surfaces every new URL via the OnURLChange callback so callers can
// persist it (so users always have a way to recover the live URL after an
// install banner goes stale).
package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
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

// Tunnel is a supervised cloudflared quick-tunnel. The watchdog goroutine
// respawns cloudflared whenever it exits until the parent context is cancelled.
type Tunnel struct {
	mu  sync.RWMutex
	url string

	cancel context.CancelFunc
	done   chan struct{}

	onChange func(string)
}

// Start launches cloudflared, waits for the first public URL, and returns once
// it is known. A watchdog goroutine then keeps the tunnel alive for the
// lifetime of ctx — if cloudflared exits, it is respawned and the new URL is
// reported via onChange (which may be nil).
//
// localURL is the address cloudflared forwards to (e.g. "http://127.0.0.1:7777").
func Start(ctx context.Context, localURL string, onChange func(string)) (*Tunnel, error) {
	bin, err := ensureCloudflared()
	if err != nil {
		return nil, fmt.Errorf("cloudflared: %w", err)
	}

	supervisorCtx, cancel := context.WithCancel(ctx)
	t := &Tunnel{cancel: cancel, done: make(chan struct{}), onChange: onChange}

	firstURL := make(chan string, 1)
	firstErr := make(chan error, 1)

	go t.supervise(supervisorCtx, bin, localURL, firstURL, firstErr)

	select {
	case url := <-firstURL:
		t.set(url)
		return t, nil
	case err := <-firstErr:
		cancel()
		<-t.done
		return nil, err
	case <-ctx.Done():
		cancel()
		<-t.done
		return nil, ctx.Err()
	}
}

// URL returns the most recently published public URL (thread-safe).
func (t *Tunnel) URL() string {
	if t == nil {
		return ""
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.url
}

// Stop terminates the tunnel and waits for the supervisor to return.
func (t *Tunnel) Stop() {
	if t == nil {
		return
	}
	t.cancel()
	<-t.done
}

// Wait blocks until the tunnel supervisor exits (i.e. the parent ctx is done).
func (t *Tunnel) Wait() error {
	if t == nil {
		return nil
	}
	<-t.done
	return nil
}

func (t *Tunnel) set(url string) {
	t.mu.Lock()
	changed := url != t.url
	t.url = url
	t.mu.Unlock()
	if changed && t.onChange != nil {
		t.onChange(url)
	}
}

// supervise keeps cloudflared running. The first publish (or hard failure)
// is reported via firstURL/firstErr; later restarts are silent except for the
// onChange callback.
func (t *Tunnel) supervise(ctx context.Context, bin, localURL string, firstURL chan<- string, firstErr chan<- error) {
	defer close(t.done)

	const minBackoff = 2 * time.Second
	const maxBackoff = 30 * time.Second
	backoff := minBackoff
	first := true

	for {
		if ctx.Err() != nil {
			return
		}

		urlCh := make(chan string, 1)
		exitCh := make(chan error, 1)
		startErr := runCloudflared(ctx, bin, localURL, urlCh, exitCh)
		if startErr != nil {
			if first {
				firstErr <- fmt.Errorf("start cloudflared: %w", startErr)
				return
			}
			log.Printf("[tunnel] cloudflared start failed: %v — retrying in %s", startErr, backoff)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		// Wait for first URL of this run (or for the process to die).
		var got string
		select {
		case got = <-urlCh:
		case err := <-exitCh:
			if first {
				firstErr <- fmt.Errorf("cloudflared exited before publishing URL: %w", err)
				return
			}
			log.Printf("[tunnel] cloudflared exited before publishing URL: %v — retrying in %s", err, backoff)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		case <-time.After(45 * time.Second):
			if first {
				firstErr <- fmt.Errorf("cloudflared did not publish a URL within 45s")
				return
			}
			log.Printf("[tunnel] cloudflared did not publish a URL within 45s — restarting")
			// Drain exit (cancel happens implicitly when ctx cancelled or via next loop)
			continue
		case <-ctx.Done():
			return
		}

		if first {
			firstURL <- got
			first = false
		} else {
			t.set(got)
			log.Printf("[tunnel] cloudflared restarted; new URL: %s", got)
		}
		backoff = minBackoff

		// Stay until the process dies or ctx cancels.
		select {
		case err := <-exitCh:
			if ctx.Err() != nil {
				return
			}
			log.Printf("[tunnel] cloudflared exited: %v — restarting in %s", err, backoff)
			if !sleepCtx(ctx, backoff) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// runCloudflared starts a single cloudflared process. Each URL printed on
// stderr is forwarded to urlCh; the process exit status lands on exitCh.
func runCloudflared(ctx context.Context, bin, localURL string, urlCh chan<- string, exitCh chan<- error) error {
	cmd := exec.CommandContext(ctx, bin,
		"tunnel",
		"--url", localURL,
		"--no-autoupdate",
		"--metrics", "127.0.0.1:0",
	)
	cmd.Stdout = io.Discard
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go scanForURL(stderr, urlCh)
	go func() { exitCh <- cmd.Wait() }()
	return nil
}

func scanForURL(r io.Reader, ch chan<- string) {
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		line := scan.Text()
		if m := trycloudflareRE.FindString(line); m != "" {
			// Non-blocking send: only the first URL of each run is consumed
			// by the supervisor; the buffer absorbs that one.
			select {
			case ch <- m:
			default:
			}
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func nextBackoff(cur, max time.Duration) time.Duration {
	n := cur * 2
	if n > max {
		return max
	}
	return n
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
