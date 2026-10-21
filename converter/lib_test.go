package converter

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	// 1. Setup Paths
	// We assume the test runs from within the 'converter' directory,
	// so the test_data is one level up.
	wd, _ := os.Getwd()
	rootDir := filepath.Dir(wd)
	fontPath := filepath.Join(rootDir, "test_data", "Go-Regular.ttf")

	// Verify font exists before running (skip if missing)
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skipf("Test font not found at %s. Run 'make fetch-test-data' first.", fontPath)
	}

	// Create a temporary output directory
	tempDir := t.TempDir()
	outPrefix := filepath.Join(tempDir, "test_output")

	// 2. Execution
	err := Generate(fontPath, 32, "ABC", outPrefix, "png", 2, "full", 0)

	// 3. Assertions
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Check if .png was created
	if _, err := os.Stat(outPrefix + ".png"); os.IsNotExist(err) {
		t.Errorf("Expected %s.png to exist", outPrefix)
	}

	// Check if .fnt was created
	if _, err := os.Stat(outPrefix + ".fnt"); os.IsNotExist(err) {
		t.Errorf("Expected %s.fnt to exist", outPrefix)
	}
}

func TestRenderTextBMP(t *testing.T) {
	wd, _ := os.Getwd()
	fontPath := filepath.Join(filepath.Dir(wd), "test_data", "Go-Regular.ttf")
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skipf("Test font not found at %s. Run 'make fetch-test-data' first.", fontPath)
	}

	outPath := filepath.Join(t.TempDir(), "sample.bmp")
	width, err := RenderTextBMP(fontPath, 16, "Hello", outPath, Options{
		Hinting:   "full",
		BPP:       1,
		Threshold: DefaultThreshold,
	})
	if err != nil {
		t.Fatalf("RenderTextBMP() failed: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading bitmap: %v", err)
	}
	if len(data) < 22 || string(data[:2]) != "BM" {
		t.Fatalf("invalid BMP output")
	}
	if got := int(binary.LittleEndian.Uint32(data[18:22])); got != width {
		t.Errorf("bitmap width = %d, reported width = %d", got, width)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(outPath), "sample.fnt")); !os.IsNotExist(err) {
		t.Errorf("text render unexpectedly created an .fnt file")
	}
}

func TestRenderTextBMPRejectsMultiline(t *testing.T) {
	_, err := RenderTextBMP("font.ttf", 16, "one\ntwo", "unused.bmp", Options{BPP: 1})
	if err == nil {
		t.Fatal("multiline text succeeded, want an error")
	}
}
