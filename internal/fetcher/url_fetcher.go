package fetcher

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AuthType represents the type of authentication for a remote URL.
type AuthType string

const (
	AuthTypeJWT   AuthType = "jwt"
	AuthTypeBasic AuthType = "basic"
	AuthTypeToken AuthType = "token"
)

// URLFetcher downloads content from remote URLs with optional authentication.
// It uses a custom dialer that blocks connections to private/loopback IP ranges
// to prevent Server-Side Request Forgery (SSRF) attacks.
type URLFetcher struct {
	client *http.Client
}

// New creates a new URLFetcher with the given HTTP timeout.
// The fetcher uses a safe dialer that blocks private IP ranges.
func New(timeout time.Duration) *URLFetcher {
	dialer := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q: %w", addr, err)
			}
			ips, err := net.LookupHost(host)
			if err != nil {
				return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
			}
			for _, ipStr := range ips {
				ip := net.ParseIP(ipStr)
				if ip == nil {
					continue
				}
				if isPrivateIP(ip) {
					return nil, fmt.Errorf("connection to private/reserved address %s is not permitted", ipStr)
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
	}

	return &URLFetcher{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// NewWithClient creates a URLFetcher that uses the provided http.Client directly.
// This is intended for testing only; it bypasses the private IP protection.
func NewWithClient(client *http.Client) *URLFetcher {
	return &URLFetcher{client: client}
}

// isPrivateIP reports whether ip falls in a private, loopback, link-local, or
// otherwise non-routable address range.
func isPrivateIP(ip net.IP) bool {
	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
		"169.254.0.0/16",
		"100.64.0.0/10",
		"0.0.0.0/8",
	}
	for _, cidr := range private {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// FetchResult holds the raw bytes and content-type from a remote resource.
type FetchResult struct {
	Body        []byte
	ContentType string
}

// validateURL checks that the URL uses an allowed scheme (http or https only)
// to prevent SSRF attacks via file://, ftp://, etc.
func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("URL scheme %q is not allowed; only http and https are permitted", scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL has no host")
	}
	return nil
}

// Fetch retrieves the content at rawURL, applying auth if provided.
// It validates the URL scheme and blocks connections to private IP ranges.
func (f *URLFetcher) Fetch(rawURL string, authType AuthType, authCredential string) (*FetchResult, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	switch authType {
	case AuthTypeJWT:
		req.Header.Set("Authorization", "Bearer "+authCredential)
	case AuthTypeToken:
		req.Header.Set("Authorization", "Bearer "+authCredential)
	case AuthTypeBasic:
		// authCredential should be "user:password" in plain text.
		encoded := base64.StdEncoding.EncodeToString([]byte(authCredential))
		req.Header.Set("Authorization", "Basic "+encoded)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	return &FetchResult{
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}
