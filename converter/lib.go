package converter

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Options controls how a font is rendered and written.
type Options struct {
	// Format is "png" or "bmp" (image + fnt), or "c" (C source + header in the
	// Microchip Graphics Library font format).
	Format string
	// Padding is the gap between glyphs in the atlas, in pixels.
	Padding int
	// Hinting is "none", "vertical" or "full".
	Hinting string
	// Stroke is 0 for filled glyphs; a positive width in pixels, which may be
	// fractional, keeps only the band that far inside each glyph's edge (hollow
	// glyphs). It changes the image only; the fnt is the same as for filled glyphs.
	Stroke float64
	// BPP is the colour depth, 1, 2, 4 or 8 bits per pixel: the number of
	// antialiasing levels is 2^BPP, so 1 turns antialiasing off.
	BPP int
	// Threshold is the coverage, above 0 and at most 1, at which a pixel is on at
	// 1 bpp. It is ignored at other depths.
	Threshold float64
}

// Generate creates the Font files (image + fnt) with full 8-bit antialiasing.
// It accepts 'hinting' ("none", "vertical", "full")
// and 'stroke': 0 draws filled glyphs; a positive width in pixels, which may be
// fractional, keeps only the band that far inside each glyph's edge (hollow glyphs).
// The stroke changes the image only; the fnt is the same as for filled glyphs.
func Generate(fontPath string, size int, chars string, outPrefix string, format string, padding int, hinting string, stroke float64) error {
	return GenerateWithOptions(fontPath, size, chars, outPrefix, Options{
		Format:    format,
		Padding:   padding,
		Hinting:   hinting,
		Stroke:    stroke,
		BPP:       8,
		Threshold: DefaultThreshold,
	})
}

// GenerateWithOptions creates the font files for one face and size: an image and an
// fnt descriptor, or for Format "c" a C source and header for an embedded target.
func GenerateWithOptions(fontPath string, size int, chars string, outPrefix string, opts Options) (err error) {
	format, padding, hinting, stroke := opts.Format, opts.Padding, opts.Hinting, opts.Stroke
	if stroke != 0 && !(stroke >= MinStroke) {
		return fmt.Errorf("stroke %v: must be 0 (filled) or at least %v px", stroke, MinStroke)
	}
	if !ValidBPP(opts.BPP) {
		return fmt.Errorf("bpp %d: must be 1, 2, 4 or 8", opts.BPP)
	}
	if opts.BPP == 1 && !ValidThreshold(opts.Threshold) {
		return fmt.Errorf("threshold %v: must be above 0 and at most 1", opts.Threshold)
	}

	// 1. Read & Parse Font
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return fmt.Errorf("reading font file: %w", err)
	}
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return fmt.Errorf("parsing font: %w", err)
	}

	// 2. Resolve Hinting Option
	var h font.Hinting
	switch hinting {
	case "none":
		h = font.HintingNone
	case "vertical":
		h = font.HintingVertical
	default:
		h = font.HintingFull // Default to crisp/sharp
	}

	// 3. Setup Font Face
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: h, // Use the selected hinting
	})
	if err != nil {
		return fmt.Errorf("creating face: %w", err)
	}
	defer func() {
		if cerr := face.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// 4. Metrics & Canvas Setup
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	lineHeight := metrics.Height.Ceil()

	var totalWidth int
	var validCharCount int

	// Measure loop
	for _, char := range chars {
		if _, advance, ok := face.GlyphBounds(char); ok {
			totalWidth += advance.Ceil() + padding
			validCharCount++
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, totalWidth, lineHeight))

	// Initialize Drawer
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.White,
		Face: face,
		Dot:  fixed.P(0, ascent),
	}

	// 5. Draw Characters Individually
	charPositions := make(map[rune]int)
	var placed []placement // every glyph drawn, duplicates included, for hollow
	currentX := 0

	for _, char := range chars {
		_, advance, ok := face.GlyphBounds(char)
		if !ok {
			continue
		}

		width := advance.Ceil()

		// Record position
		charPositions[char] = currentX
		placed = append(placed, placement{char, currentX})

		// CRITICAL FIX: Explicitly set the Dot to the exact integer position.
		// This prevents sub-pixel accumulation errors (drifting) and ensures
		// the image pixels align 1:1 with the FNT coordinates.
		drawer.Dot = fixed.P(currentX, ascent)

		// Draw
		drawer.DrawString(string(char))

		// Advance local integer tracker
		currentX += width + padding
	}

	// 5b. Hollow the glyphs out, if asked
	if stroke > 0 {
		if err := hollow(img, f, size, h, placed, ascent, stroke); err != nil {
			return fmt.Errorf("stroking glyphs: %w", err)
		}
	}

	// 5c. Reduce the antialiasing to the target depth
	if err := quantize(img, opts.BPP, opts.Threshold); err != nil {
		return err
	}

	if format == "c" {
		glyphs := make(map[rune]cGlyph, len(charPositions))
		for char, x := range charPositions {
			_, advance, _ := face.GlyphBounds(char)
			glyphs[char] = cGlyph{x: x, width: advance.Ceil()}
		}
		return writeCSource(outPrefix, img, cFont{
			fontPath:  fontPath,
			size:      size,
			hinting:   hinting,
			stroke:    stroke,
			bpp:       opts.BPP,
			threshold: opts.Threshold,
			height:    lineHeight,
			glyphs:    glyphs,
		})
	}

	// 6. Save Image
	ext := "." + format
	if err := func() error {
		imgFile, err := os.Create(outPrefix + ext)
		if err != nil {
			return err
		}
		defer func() {
			cerr := imgFile.Close()
			if err == nil {
				err = cerr
			}
		}()

		if format == "bmp" {
			if err := EncodeBMP(imgFile, img); err != nil {
				return err
			}
		} else {
			if err := png.Encode(imgFile, img); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		return err
	}

	// 7. Save FNT Data
	if err := func() error {
		fntFile, err := os.Create(outPrefix + ".fnt")
		if err != nil {
			return err
		}
		defer func() {
			cerr := fntFile.Close()
			if err == nil {
				err = cerr
			}
		}()

		fileName := filepath.Base(outPrefix) + ext

		smooth := 1
		if opts.BPP == 1 {
			smooth = 0
		}
		if _, err := fmt.Fprintf(fntFile, "info face=\"%s\" size=%d bold=0 italic=0 charset=\"\" unicode=0 stretchH=100 smooth=%d aa=1 padding=0,0,0,0 spacing=%d,1\n", filepath.Base(fontPath), size, smooth, padding); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(fntFile, "common lineHeight=%d base=%d scaleW=%d scaleH=%d pages=1 packed=0\n", lineHeight, ascent, totalWidth, lineHeight); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(fntFile, "page id=0 file=\"%s\"\n", fileName); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(fntFile, "chars count=%d\n", validCharCount); err != nil {
			return err
		}

		for _, char := range chars {
			_, advance, ok := face.GlyphBounds(char)
			if !ok {
				continue
			}
			width := advance.Ceil()
			xPos := charPositions[char]

			if _, err := fmt.Fprintf(fntFile, "char id=%d x=%d y=0 width=%d height=%d xoffset=0 yoffset=0 xadvance=%d page=0 chnl=15\n",
				char, xPos, width, lineHeight, width); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		return err
	}

	return nil
}
