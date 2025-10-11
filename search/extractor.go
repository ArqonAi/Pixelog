package search

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type TextExtractor interface {
	Extract(reader io.Reader, filename string) (string, error)
	SupportedExtensions() []string
}

type PlainTextExtractor struct{}

func NewPlainTextExtractor() *PlainTextExtractor {
	return &PlainTextExtractor{}
}

func (e *PlainTextExtractor) Extract(reader io.Reader, filename string) (string, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read content: %w", err)
	}
	return string(content), nil
}

func (e *PlainTextExtractor) SupportedExtensions() []string {
	return []string{".txt", ".md", ".csv", ".json", ".yaml", ".yml", ".log"}
}

// TODO: Implement additional extractors
type PDFExtractor struct{}
type DOCXExtractor struct{}
type ImageExtractor struct{} // OCR-based

type MultiExtractor struct {
	extractors map[string]TextExtractor
}

func NewMultiExtractor() *MultiExtractor {
	me := &MultiExtractor{
		extractors: make(map[string]TextExtractor),
	}
	
	// Register plain text extractor
	plainText := NewPlainTextExtractor()
	for _, ext := range plainText.SupportedExtensions() {
		me.extractors[ext] = plainText
	}
	
	// TODO: Register other extractors
	// pdf := NewPDFExtractor()
	// docx := NewDOCXExtractor()
	// image := NewImageExtractor()
	
	return me
}

func (me *MultiExtractor) Extract(reader io.Reader, filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	
	extractor, exists := me.extractors[ext]
	if !exists {
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
	
	return extractor.Extract(reader, filename)
}

func (me *MultiExtractor) SupportedExtensions() []string {
	var extensions []string
	for ext := range me.extractors {
		extensions = append(extensions, ext)
	}
	return extensions
}

func (me *MultiExtractor) IsSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, exists := me.extractors[ext]
	return exists
}
