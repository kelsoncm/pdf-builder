package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// PDFEngine wraps wkhtmltopdf invocation.
type PDFEngine struct {
	binaryPath string
	timeout    time.Duration
}

// NewPDFEngine creates a new PDFEngine.
func NewPDFEngine(binaryPath string, timeoutSeconds int) *PDFEngine {
	if binaryPath == "" {
		binaryPath = "wkhtmltopdf"
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	return &PDFEngine{
		binaryPath: binaryPath,
		timeout:    time.Duration(timeoutSeconds) * time.Second,
	}
}

// ConvertHTMLBytes converts HTML content (provided as bytes) to a PDF and returns
// the PDF bytes. The caller is responsible for providing a writable tmpDir;
// temporary files will be created inside it and removed after use.
func (e *PDFEngine) ConvertHTMLBytes(tmpDir string, htmlContent []byte, options map[string]string) ([]byte, error) {
	// Write HTML to a temp file.
	htmlFile, err := os.CreateTemp(tmpDir, "input-*.html")
	if err != nil {
		return nil, fmt.Errorf("creating temp HTML file: %w", err)
	}
	defer os.Remove(htmlFile.Name())

	if _, err := htmlFile.Write(htmlContent); err != nil {
		htmlFile.Close()
		return nil, fmt.Errorf("writing HTML to temp file: %w", err)
	}
	htmlFile.Close()

	return e.convertFile(tmpDir, htmlFile.Name(), options)
}

// ConvertURL fetches a URL and converts the result to PDF.
func (e *PDFEngine) ConvertURL(tmpDir string, url string, options map[string]string) ([]byte, error) {
	return e.convertFile(tmpDir, url, options)
}

// convertFile invokes wkhtmltopdf on the given input (file path or URL) and returns PDF bytes.
func (e *PDFEngine) convertFile(tmpDir, input string, options map[string]string) ([]byte, error) {
	outFile := filepath.Join(tmpDir, fmt.Sprintf("output-%d.pdf", time.Now().UnixNano()))
	defer os.Remove(outFile)

	args := buildArgs(options, input, outFile)

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("wkhtmltopdf timed out after %s", e.timeout)
		}
		return nil, fmt.Errorf("wkhtmltopdf failed: %w — output: %s", err, string(out))
	}

	pdfBytes, err := os.ReadFile(outFile)
	if err != nil {
		return nil, fmt.Errorf("reading generated PDF: %w", err)
	}
	return pdfBytes, nil
}

// buildArgs constructs the wkhtmltopdf argument list.
func buildArgs(options map[string]string, input, output string) []string {
	args := []string{"--quiet"}

	optionMap := map[string]string{
		"page_size":   "--page-size",
		"orientation": "--orientation",
		"margin_top":  "--margin-top",
		"margin_bottom": "--margin-bottom",
		"margin_left": "--margin-left",
		"margin_right": "--margin-right",
		"encoding":    "--encoding",
		"dpi":         "--dpi",
	}

	for key, value := range options {
		if flag, ok := optionMap[key]; ok {
			args = append(args, flag, value)
		}
	}

	args = append(args, input, output)
	return args
}
