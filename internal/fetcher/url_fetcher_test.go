package fetcher_test

import (
"net/http"
"net/http/httptest"
"strings"
"testing"
"time"

"github.com/kelsoncm/pdf-builder/internal/fetcher"
)

func TestFetch_BlocksFileScheme(t *testing.T) {
f := fetcher.New(5 * time.Second)
_, err := f.Fetch("file:///etc/passwd", "", "")
if err == nil {
t.Error("expected error for file:// scheme")
}
}

func TestFetch_BlocksEmptyScheme(t *testing.T) {
f := fetcher.New(5 * time.Second)
_, err := f.Fetch("/relative/path", "", "")
if err == nil {
t.Error("expected error for relative URL")
}
}

func TestFetch_BlocksPrivateIP(t *testing.T) {
f := fetcher.New(5 * time.Second)
_, err := f.Fetch("http://127.0.0.1:9999/test", "", "")
if err == nil {
t.Error("expected error for loopback address")
}
}

func TestFetch_BlocksInternalRange(t *testing.T) {
f := fetcher.New(5 * time.Second)
_, err := f.Fetch("http://192.168.1.1/admin", "", "")
if err == nil {
t.Error("expected error for private IP range")
}
}

func TestFetch_HTTPSAllowed(t *testing.T) {
// Validate that the URL scheme check passes for https
// (network failure is acceptable; scheme rejection is not).
_, err := fetcher.New(5 * time.Second).Fetch("https://example.com", "", "")
if err != nil {
if err.Error() == "URL scheme \"https\" is not allowed; only http and https are permitted" {
t.Error("https scheme should be allowed")
}
}
}

// newTestFetcher creates a URLFetcher that uses the given http.Client, bypassing
// the private IP protection so that local test servers can be reached.
func newTestFetcher(client *http.Client) *fetcher.URLFetcher {
return fetcher.NewWithClient(client)
}

func TestFetch_HTTPAllowed(t *testing.T) {
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "text/html")
w.Write([]byte("<html><body>test</body></html>"))
}))
defer ts.Close()

f := newTestFetcher(ts.Client())
result, err := f.Fetch(ts.URL, "", "")
if err != nil {
t.Fatalf("expected success, got error: %v", err)
}
if result == nil || len(result.Body) == 0 {
t.Error("expected non-empty body")
}
}

func TestFetch_WithBearerAuth(t *testing.T) {
const wantToken = "my-test-jwt-token"
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
authHeader := r.Header.Get("Authorization")
if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != wantToken {
w.WriteHeader(http.StatusUnauthorized)
return
}
w.Header().Set("Content-Type", "text/html")
w.Write([]byte("<html/>"))
}))
defer ts.Close()

f := newTestFetcher(ts.Client())
result, err := f.Fetch(ts.URL, fetcher.AuthTypeJWT, wantToken)
if err != nil {
t.Fatalf("expected success, got: %v", err)
}
if len(result.Body) == 0 {
t.Error("expected non-empty body")
}
}
