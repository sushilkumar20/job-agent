package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html"
)

const maxPageBytes = 512 << 10

// Fetcher performs the network call behind the fetch_url tool. The caller
// supplies a URL chosen by a language model, so the client refuses anything
// on the local network rather than trusting the address it is given.
type Fetcher struct {
	http *http.Client
}

func NewFetcher() *Fetcher {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		// Control runs after DNS resolution and before connect, so a hostname
		// that resolves to a private address is blocked here — a check on the
		// URL string alone would miss that.
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("could not parse address %q", host)
			}
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
				return fmt.Errorf("refusing to connect to non-public address %s", ip)
			}
			return nil
		},
	}

	return &Fetcher{
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{DialContext: dialer.DialContext},
		},
	}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("not a valid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("only http and https URLs are supported")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	// Many sites reject requests with no User-Agent outright.
	req.Header.Set("User-Agent", "jobagent/0.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json")

	resp, err := f.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned %s", parsed.Host, resp.Status)
	}

	body := io.LimitReader(resp.Body, maxPageBytes)

	// JSON endpoints must not go through the HTML parser — the structure is
	// the useful part.
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		raw, err := io.ReadAll(body)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}

	text, err := extractText(body)
	if err != nil {
		return "", fmt.Errorf("parse page: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("no readable text found at %s (the page may render via JavaScript)", parsed.Host)
	}
	return text, nil
}

// skipTags hold content that is markup or code rather than prose.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true,
	"svg": true, "head": true, "nav": true, "footer": true,
}

func extractText(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			if s := strings.TrimSpace(n.Data); s != "" {
				b.WriteString(s)
				b.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return collapseBlankLines(b.String()), nil
}

func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
