package response

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// NamedPDF pairs a filename with its PDF bytes.
type NamedPDF struct {
	Name string
	Data []byte
}

// Send writes one or more PDFs to the Gin response.
// If there is exactly one PDF, it is returned as application/pdf.
// If there are two or more PDFs, they are packaged into a ZIP archive.
func Send(c *gin.Context, pdfs []NamedPDF) error {
	switch len(pdfs) {
	case 0:
		return fmt.Errorf("no PDFs to send")
	case 1:
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, pdfs[0].Name))
		c.Data(http.StatusOK, "application/pdf", pdfs[0].Data)
		return nil
	default:
		return sendZIP(c, pdfs)
	}
}

func sendZIP(c *gin.Context, pdfs []NamedPDF) error {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, p := range pdfs {
		name := p.Name
		if len(name) < 4 || name[len(name)-4:] != ".pdf" {
			name += ".pdf"
		}
		f, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("creating zip entry %q: %w", name, err)
		}
		if _, err := f.Write(p.Data); err != nil {
			return fmt.Errorf("writing zip entry %q: %w", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("closing zip writer: %w", err)
	}

	c.Header("Content-Disposition", `attachment; filename="results.zip"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
	return nil
}
