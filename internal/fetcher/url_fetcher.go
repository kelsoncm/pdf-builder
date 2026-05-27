package fetcher

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type AuthType string

const (
	AuthTypeJWT   AuthType = "jwt"
	AuthTypeBasic AuthType = "basic"
	AuthTypeToken AuthType = "token"
)

type URLFetcher struct {
	client *http.Client
}

func New(timeout time.Duration) *URLFetcher {
	dialer := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q: %w", addr, err)
			}

			ips, err := net.LookupIP(host)
			if err != nil {
				return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
			}

			for _, ip := range ips {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("connection to private/reserved address %s is not permitted", ip.String())
				}
			}

			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
	}

	return &URLFetcher{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func NewWithClient(client *http.Client) *URLFetcher {
	return &URLFetcher{client: client}
}

func isBlockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}

	return addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified()
}

type FetchResult struct {
	Body        []byte
	ContentType string
}

func validateURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("URL scheme %q is not allowed; only http and https are permitted", scheme)
	}

	if parsed.Host == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("URL has no host")
	}

	if parsed.User != nil {
		return nil, fmt.Errorf("userinfo in URL is not allowed")
	}

	port := parsed.Port()
	if port != "" && port != "80" && port != "443" {
		return nil, fmt.Errorf("port %q is not allowed", port)
	}

	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil, fmt.Errorf("localhost is not allowed")
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}

	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("connection to private/reserved address %s is not permitted", ip.String())
		}
	}

	return parsed, nil
}

func (f *URLFetcher) Fetch(rawURL string, authType AuthType, authCredential string) (*FetchResult, error) {
	parsed, err := validateURL(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	switch authType {
	case AuthTypeJWT, AuthTypeToken:
		req.Header.Set("Authorization", "Bearer "+authCredential)
	case AuthTypeBasic:
		encoded := base64.StdEncoding.EncodeToString([]byte(authCredential))
		req.Header.Set("Authorization", "Basic "+encoded)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, fmt.Errorf("redirects are not allowed")
	}

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
