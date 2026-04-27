package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"infracanvas/pkg/runstate"
)

var (
	urlNoToken bool
	urlRaw     bool
)

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "Print the current public dashboard URL",
	Long: `Print the current public URL with auth token.

The Cloudflare quick-tunnel hostname changes whenever cloudflared restarts —
which happens on service restart, on transient network drops, and at random
under Cloudflare's free-tier limits. This command reads the live URL from the
state file written by ` + "`infracanvas serve`" + ` so you always have a working link.

If the state file is missing (e.g. after upgrading from an older version),
this command falls back to scraping the most recent URL from the systemd
journal.`,
	RunE: runURL,
}

func init() {
	rootCmd.AddCommand(urlCmd)
	urlCmd.Flags().BoolVar(&urlNoToken, "no-token", false, "Print the bare URL without ?token=…")
	urlCmd.Flags().BoolVar(&urlRaw, "raw", false, "Print only the URL (no surrounding text)")
}

func runURL(_ *cobra.Command, _ []string) error {
	s, err := runstate.Read()
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	url := s.TunnelURL
	if url == "" {
		url = scrapeJournalURL()
	}
	if url == "" {
		return fmt.Errorf("no public URL recorded yet — is `infracanvas serve` running with the tunnel enabled? Try: sudo systemctl status infracanvas")
	}

	if !urlNoToken && s.Token != "" {
		url = fmt.Sprintf("%s/?token=%s", strings.TrimRight(url, "/"), s.Token)
	}

	if urlRaw {
		fmt.Println(url)
		return nil
	}

	fmt.Println()
	fmt.Println("  Open in your browser:")
	fmt.Printf("    \033[1;36m%s\033[0m\n", url)
	fmt.Println()
	fmt.Println("  This URL is regenerated on every cloudflared restart. Re-run")
	fmt.Println("  `infracanvas url` any time to fetch the current one.")
	fmt.Println()
	return nil
}

var trycloudflareLine = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// scrapeJournalURL is a best-effort fallback for older installs that pre-date
// the state file. Returns "" if journalctl isn't available or finds nothing.
func scrapeJournalURL() string {
	if _, err := exec.LookPath("journalctl"); err != nil {
		return ""
	}
	out, err := exec.Command("journalctl", "-u", "infracanvas", "--no-pager", "-n", "500").Output()
	if err != nil {
		return ""
	}
	matches := trycloudflareLine.FindAllString(string(out), -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}
