package fetcher

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
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

// Fetch retrieves the content at url, applying auth if provided.
func (f *URLFetcher) Fetch(url string, authType AuthType, authCredential string) (*FetchResult, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %q: %w", url, err)
	}

	switch authType {
	case AuthTypeJWT:
		req.Header.Set("Authorization", "Bearer "+authCredential)
	case AuthTypeToken:
		req.Header.Set("Authorization", "Bearer "+authCredential)
	case AuthTypeBasic:
		// authCredential should be "user:password" in plain text or already Base64.
		encoded := base64.StdEncoding.EncodeToString([]byte(authCredential))
		req.Header.Set("Authorization", "Basic "+encoded)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote server returned %d for %q", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body from %q: %w", url, err)
	}

	return &FetchResult{
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}
