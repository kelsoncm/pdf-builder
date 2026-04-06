package response_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kelsoncm/pdf-builder/internal/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSend_SinglePDF(t *testing.T) {
	r := gin.New()
	pdfData := []byte("%PDF-1.4 test content")
	r.GET("/test", func(c *gin.Context) {
		_ = response.Send(c, []response.NamedPDF{{Name: "doc", Data: pdfData}})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected application/pdf, got %q", ct)
	}
	if !bytes.Equal(w.Body.Bytes(), pdfData) {
		t.Error("PDF body does not match")
	}
}

func TestSend_MultiplePDFs_ZIP(t *testing.T) {
	r := gin.New()
	pdfs := []response.NamedPDF{
		{Name: "doc1", Data: []byte("%PDF-1.4 doc1")},
		{Name: "doc2", Data: []byte("%PDF-1.4 doc2")},
	}
	r.GET("/test", func(c *gin.Context) {
		_ = response.Send(c, pdfs)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected application/zip, got %q", ct)
	}

	// Verify zip contents.
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Errorf("expected 2 files in zip, got %d", len(zr.File))
	}
	for i, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		if !bytes.Equal(data, pdfs[i].Data) {
			t.Errorf("zip entry %d data mismatch", i)
		}
	}
}

func TestSend_NoPDFs(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(c *gin.Context) {
		err := response.Send(c, nil)
		if err == nil {
			t.Error("expected error for empty PDFs")
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
}
