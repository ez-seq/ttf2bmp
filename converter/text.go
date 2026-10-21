package converter

import (
	"fmt"
	"image"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// RenderTextBMP renders a single line without inter-glyph padding and writes it as BMP.
// The returned width is the sum of the glyph advances, matching the MLA font layout.
func RenderTextBMP(fontPath string, size int, text, outPath string, opts Options) (width int, err error) {
	if size <= 0 {
		return 0, fmt.Errorf("size must be positive")
	}
	if text == "" {
		return 0, fmt.Errorf("text must not be empty")
	}
	if strings.ContainsAny(text, "\r\n") {
		return 0, fmt.Errorf("text must be a single line")
	}
	if opts.Stroke != 0 && !(opts.Stroke >= MinStroke) {
		return 0, fmt.Errorf("stroke %v: must be 0 (filled) or at least %v px", opts.Stroke, MinStroke)
	}
	if !ValidBPP(opts.BPP) {
		return 0, fmt.Errorf("bpp %d: must be 1, 2, 4 or 8", opts.BPP)
	}
	if opts.BPP == 1 && !ValidThreshold(opts.Threshold) {
		return 0, fmt.Errorf("threshold %v: must be above 0 and at most 1", opts.Threshold)
	}

	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return 0, fmt.Errorf("reading font file: %w", err)
	}
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return 0, fmt.Errorf("parsing font: %w", err)
	}

	var hinting font.Hinting
	switch opts.Hinting {
	case "none":
		hinting = font.HintingNone
	case "vertical":
		hinting = font.HintingVertical
	default:
		hinting = font.HintingFull
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: hinting,
	})
	if err != nil {
		return 0, fmt.Errorf("creating face: %w", err)
	}
	defer func() {
		if cerr := face.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	ascent := face.Metrics().Ascent.Ceil()
	lineHeight := face.Metrics().Height.Ceil()
	if opts.Tight {
		if top, bottom, ok := inkExtent(face, text); ok {
			ascent = -top
			lineHeight = bottom - top
		}
	}

	placed := make([]placement, 0, len(text))
	for _, char := range text {
		if char == '\t' {
			return 0, fmt.Errorf("tab characters are not supported")
		}
		_, advance, ok := face.GlyphBounds(char)
		if !ok {
			return 0, fmt.Errorf("font has no glyph for %q", char)
		}
		placed = append(placed, placement{char: char, x: width})
		width += advance.Ceil()
	}
	if width == 0 {
		return 0, fmt.Errorf("text has zero pixel advance")
	}

	img := image.NewRGBA(image.Rect(0, 0, width, lineHeight))
	drawer := &font.Drawer{Dst: img, Src: image.White, Face: face}
	for _, p := range placed {
		drawer.Dot = fixed.P(p.x, ascent)
		drawer.DrawString(string(p.char))
	}
	if opts.Stroke > 0 {
		if err := hollow(img, f, size, hinting, placed, ascent, opts.Stroke); err != nil {
			return 0, fmt.Errorf("stroking text: %w", err)
		}
	}
	if err := quantize(img, opts.BPP, opts.Threshold); err != nil {
		return 0, err
	}
	if opts.Tight {
		img, _ = trimRows(img)
	}

	file, err := os.Create(outPath)
	if err != nil {
		return 0, fmt.Errorf("creating bitmap: %w", err)
	}
	if err = EncodeBMP(file, img); err != nil {
		_ = file.Close()
		return 0, fmt.Errorf("writing bitmap: %w", err)
	}
	if err = file.Close(); err != nil {
		return 0, fmt.Errorf("closing bitmap: %w", err)
	}
	return width, nil
}
