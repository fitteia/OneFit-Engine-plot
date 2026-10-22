package main

import (
	"errors"
	"testing"
)

func TestMergeErrorNamesTheFile(t *testing.T) {
	files := []string{"fit-curves-1.pdf", "fit-curves-2.pdf"}
	for in, want := range map[string]string{
		"PDF 2: encrypted PDF files are not supported": "fit-curves-2.pdf: encrypted PDF files are not supported",
		"PDF 3: out of range":                          "PDF 3: out of range",
		"no PDF files to merge":                        "no PDF files to merge",
	} {
		if got := mergeError(errors.New(in), files).Error(); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
