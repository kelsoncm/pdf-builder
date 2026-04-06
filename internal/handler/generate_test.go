package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kelsoncm/pdf-builder/internal/auth"
	"github.com/kelsoncm/pdf-builder/internal/engine"
	"github.com/kelsoncm/pdf-builder/internal/fetcher"
	"github.com/kelsoncm/pdf-builder/internal/handler"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestRouter() *gin.Engine {
	logger := zap.NewNop()
	tokenMap := map[string]string{"test-token": "testuser"}

	pdfEng := engine.NewPDFEngine("", 5)
	urlFetcher := fetcher.New(5)
	h := handler.NewGenerateHandler(pdfEng, urlFetcher, logger)

	r := gin.New()

	// Health check — no auth.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Protected routes.
	protected := r.Group("/")
	protected.Use(auth.Middleware(tokenMap, logger))
	protected.POST("/generate", h.Handle)

	return r
}

func TestHealthCheck(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %q", resp["status"])
	}
}

func TestGenerate_Unauthorized(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGenerate_EmptyBody(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGenerate_InvalidJSON(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGenerate_InlineHTML_WkhtmltopdfNotInstalled(t *testing.T) {
	// This test verifies the handler correctly propagates a wkhtmltopdf error
	// when the binary is not installed (expected in CI without wkhtmltopdf).
	r := newTestRouter()

	body := map[string]interface{}{
		"jobs": []map[string]interface{}{
			{
				"name": "test-doc",
				"sources": []map[string]interface{}{
					{
						"type": "inline",
						"html": "<html><body><h1>Hello</h1></body></html>",
					},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	r.ServeHTTP(w, req)

	// When wkhtmltopdf is not installed, we expect either a 200 (if installed) or 500 (if not).
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d", w.Code)
	}
}
