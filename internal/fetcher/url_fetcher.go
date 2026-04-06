package fetcher

import (
	"encoding/base64"
	"fmt"
	"io"
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
type URLFetcher struct {
	client *http.Client
}

// New creates a new URLFetcher with the given HTTP timeout.
func New(timeout time.Duration) *URLFetcher {
	return &URLFetcher{
		client: &http.Client{Timeout: timeout},
	}
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
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("URL scheme %q is not allowed; only http and https are permitted", scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL %q has no host", rawURL)
	}
	return nil
}

// Fetch retrieves the content at rawURL, applying auth if provided.
func (f *URLFetcher) Fetch(rawURL string, authType AuthType, authCredential string) (*FetchResult, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for URL: %w", err)
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
