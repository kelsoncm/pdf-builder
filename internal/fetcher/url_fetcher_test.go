package fetcher_test

import (
"net/http"
"net/http/httptest"
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

func TestFetch_HTTPSAllowed(t *testing.T) {
// Validate that the URL scheme check passes for https
// (network failure is acceptable; scheme rejection is not).
_, err := fetcher.New(5 * time.Second).Fetch("https://example.com", "", "")
if err != nil {
errStr := err.Error()
if errStr == "URL scheme \"https\" is not allowed; only http and https are permitted" {
t.Error("https scheme should be allowed")
}
}
}

func TestFetch_HTTPAllowed(t *testing.T) {
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "text/html")
w.Write([]byte("<html><body>test</body></html>"))
}))
defer ts.Close()

f := fetcher.New(5 * time.Second)
result, err := f.Fetch(ts.URL, "", "")
if err != nil {
t.Fatalf("expected success, got error: %v", err)
}
if result == nil || len(result.Body) == 0 {
t.Error("expected non-empty body")
}
}

func TestFetch_WithBearerAuth(t *testing.T) {
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
auth := r.Header.Get("Authorization")
if auth != "Bearer my-jwt-token" {
w.WriteHeader(http.StatusUnauthorized)
return
}
w.Header().Set("Content-Type", "text/html")
w.Write([]byte("<html/>"))
}))
defer ts.Close()

f := fetcher.New(5 * time.Second)
result, err := f.Fetch(ts.URL, fetcher.AuthTypeJWT, "my-jwt-token")
if err != nil {
t.Fatalf("expected success, got: %v", err)
}
if len(result.Body) == 0 {
t.Error("expected non-empty body")
}
}
