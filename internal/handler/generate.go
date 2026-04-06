package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kelsoncm/pdf-builder/internal/auth"
	"github.com/kelsoncm/pdf-builder/internal/engine"
	"github.com/kelsoncm/pdf-builder/internal/fetcher"
	"github.com/kelsoncm/pdf-builder/internal/response"
)

// Source types.
const (
	SourceTypeInline = "inline"
	SourceTypeURL    = "url"
)

// Source represents one HTML/URL input for a job.
type Source struct {
	Type           string `json:"type"`
	HTML           string `json:"html,omitempty"`
	URL            string `json:"url,omitempty"`
	AuthType       string `json:"auth_type,omitempty"`
	AuthCredential string `json:"auth_credential,omitempty"`
}

// Job is a single PDF generation unit.
type Job struct {
	Name               string            `json:"name"`
	Sources            []Source          `json:"sources"`
	WkhtmltopdfOptions map[string]string `json:"wkhtmltopdf_options,omitempty"`
}

// MergeInstruction describes how to combine job PDFs.
type MergeInstruction struct {
	Name      string   `json:"name"`
	InputJobs []string `json:"input_jobs"`
}

// OutputOptions controls what appears in the response.
type OutputOptions struct {
	IncludeJobs   bool `json:"include_jobs"`
	IncludeMerges bool `json:"include_merges"`
}

// GenerateRequest is the top-level POST /generate body.
type GenerateRequest struct {
	Jobs   []Job              `json:"jobs"`
	Merge  []MergeInstruction `json:"merge,omitempty"`
	Output *OutputOptions     `json:"output,omitempty"`
}

// GenerateHandler holds the dependencies for the /generate endpoint.
type GenerateHandler struct {
	pdfEngine *engine.PDFEngine
	fetcher   *fetcher.URLFetcher
	logger    *zap.Logger
}

// NewGenerateHandler constructs a GenerateHandler.
func NewGenerateHandler(pdfEngine *engine.PDFEngine, f *fetcher.URLFetcher, logger *zap.Logger) *GenerateHandler {
	return &GenerateHandler{
		pdfEngine: pdfEngine,
		fetcher:   f,
		logger:    logger,
	}
}

// Handle processes POST /generate requests.
func (h *GenerateHandler) Handle(c *gin.Context) {
	username := auth.Username(c)
	log := h.logger.With(zap.String("username", username))

	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Jobs) == 0 && len(req.Merge) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request must contain at least one job or merge instruction"})
		return
	}

	// Default output options.
	outOpts := OutputOptions{IncludeJobs: false, IncludeMerges: true}
	if req.Output != nil {
		outOpts = *req.Output
	}

	// Create a temporary directory for this request.
	tmpDir, err := os.MkdirTemp("", "pdf-builder-*")
	if err != nil {
		log.Error("creating temp dir", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer os.RemoveAll(tmpDir)

	// Process jobs.
	jobResults := make(map[string][]byte, len(req.Jobs))
	for _, job := range req.Jobs {
		pdf, err := h.processJob(log, tmpDir, job)
		if err != nil {
			log.Error("processing job", zap.String("job", job.Name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("job %q failed: %s", job.Name, err.Error())})
			return
		}
		jobResults[job.Name] = pdf
	}

	// Process merge instructions.
	mergeResults := make(map[string][]byte, len(req.Merge))
	for _, mi := range req.Merge {
		merged, err := h.processMerge(mi, jobResults)
		if err != nil {
			log.Error("processing merge", zap.String("merge", mi.Name), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("merge %q failed: %s", mi.Name, err.Error())})
			return
		}
		mergeResults[mi.Name] = merged
	}

	// Assemble output.
	var pdfs []response.NamedPDF
	if outOpts.IncludeJobs {
		for _, job := range req.Jobs {
			if data, ok := jobResults[job.Name]; ok {
				pdfs = append(pdfs, response.NamedPDF{Name: job.Name, Data: data})
			}
		}
	}
	if outOpts.IncludeMerges {
		for _, mi := range req.Merge {
			if data, ok := mergeResults[mi.Name]; ok {
				pdfs = append(pdfs, response.NamedPDF{Name: mi.Name, Data: data})
			}
		}
	}

	// If no explicit output selection, return all job PDFs.
	if len(pdfs) == 0 {
		for _, job := range req.Jobs {
			if data, ok := jobResults[job.Name]; ok {
				pdfs = append(pdfs, response.NamedPDF{Name: job.Name, Data: data})
			}
		}
	}

	if len(pdfs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no output PDFs produced"})
		return
	}

	if err := response.Send(c, pdfs); err != nil {
		log.Error("sending response", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build response"})
	}
}

// processJob generates a single PDF for a job by processing all its sources
// and merging them if there are multiple.
func (h *GenerateHandler) processJob(log *zap.Logger, tmpDir string, job Job) ([]byte, error) {
	if len(job.Sources) == 0 {
		return nil, fmt.Errorf("job has no sources")
	}

	var sourcePDFs [][]byte
	for _, src := range job.Sources {
		pdf, err := h.processSource(log, tmpDir, src, job.WkhtmltopdfOptions)
		if err != nil {
			return nil, fmt.Errorf("source processing failed: %w", err)
		}
		sourcePDFs = append(sourcePDFs, pdf)
	}

	if len(sourcePDFs) == 1 {
		return sourcePDFs[0], nil
	}

	return engine.MergePDFs(sourcePDFs)
}

// processSource generates a PDF from a single source.
func (h *GenerateHandler) processSource(log *zap.Logger, tmpDir string, src Source, opts map[string]string) ([]byte, error) {
	switch strings.ToLower(src.Type) {
	case SourceTypeInline:
		if src.HTML == "" {
			return nil, fmt.Errorf("inline source has empty HTML")
		}
		return h.pdfEngine.ConvertHTMLBytes(tmpDir, []byte(src.HTML), opts)

	case SourceTypeURL:
		if src.URL == "" {
			return nil, fmt.Errorf("url source has empty URL")
		}
		authType := fetcher.AuthType(strings.ToLower(src.AuthType))
		result, err := h.fetcher.Fetch(src.URL, authType, src.AuthCredential)
		if err != nil {
			return nil, fmt.Errorf("fetching URL %q: %w", src.URL, err)
		}

		// If the remote resource is already a PDF, use it directly.
		ct := strings.ToLower(result.ContentType)
		if strings.Contains(ct, "application/pdf") {
			log.Debug("URL returned a PDF directly", zap.String("url", src.URL))
			return result.Body, nil
		}

		// Otherwise treat it as HTML and convert.
		return h.pdfEngine.ConvertHTMLBytes(tmpDir, result.Body, opts)

	default:
		return nil, fmt.Errorf("unknown source type %q", src.Type)
	}
}

// processMerge combines PDFs from previously generated jobs.
func (h *GenerateHandler) processMerge(mi MergeInstruction, jobResults map[string][]byte) ([]byte, error) {
	if len(mi.InputJobs) == 0 {
		return nil, fmt.Errorf("merge instruction has no input jobs")
	}

	inputs := make([][]byte, 0, len(mi.InputJobs))
	for _, name := range mi.InputJobs {
		data, ok := jobResults[name]
		if !ok {
			return nil, fmt.Errorf("job %q not found in results", name)
		}
		inputs = append(inputs, data)
	}

	return engine.MergePDFs(inputs)
}
