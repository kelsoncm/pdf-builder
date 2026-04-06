package engine

import (
	"bytes"
	"fmt"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// MergePDFs combines multiple PDF byte slices into a single PDF.
// The order of the input slices determines the order in the output.
func MergePDFs(inputs [][]byte) ([]byte, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no PDF inputs provided for merge")
	}
	if len(inputs) == 1 {
		return inputs[0], nil
	}

	readers := make([]io.ReadSeeker, len(inputs))
	for i, b := range inputs {
		readers[i] = bytes.NewReader(b)
	}

	var out bytes.Buffer
	conf := model.NewDefaultConfiguration()
	if err := api.MergeRaw(readers, &out, false, conf); err != nil {
		return nil, fmt.Errorf("merging PDFs: %w", err)
	}
	return out.Bytes(), nil
}
