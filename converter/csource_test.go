package converter

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestLevel(t *testing.T) {
	cases := []struct {
		a         uint8
		bpp       int
		threshold float64
		want      uint8
	}{
		{0, 1, 0.5, 0}, {127, 1, 0.5, 0}, {128, 1, 0.5, 1}, {255, 1, 0.5, 1},
		{1, 1, 1.0 / 255, 1}, {254, 1, 1, 0}, {255, 1, 1, 1},
		{0, 2, 0, 0}, {42, 2, 0, 0}, {43, 2, 0, 1}, {128, 2, 0, 2}, {255, 2, 0, 3},
		{0, 4, 0, 0}, {8, 4, 0, 0}, {9, 4, 0, 1}, {255, 4, 0, 15},
		{0, 8, 0, 0}, {77, 8, 0, 77}, {255, 8, 0, 255},
	}
	for _, c := range cases {
		if got := level(c.a, c.bpp, c.threshold); got != c.want {
			t.Errorf("level(%d, %d bpp, %v) = %d, want %d", c.a, c.bpp, c.threshold, got, c.want)
		}
	}
}

func TestQuantizeLevels(t *testing.T) {
	for _, bpp := range []int{1, 2, 4, 8} {
		img := image.NewRGBA(image.Rect(0, 0, 256, 1))
		for x := 0; x < 256; x++ {
			v := uint8(x)
			img.Pix[x*4], img.Pix[x*4+1], img.Pix[x*4+2], img.Pix[x*4+3] = v, v, v, v
		}
		if err := quantize(img, bpp, DefaultThreshold); err != nil {
			t.Fatal(err)
		}
		seen := map[uint8]bool{}
		for x := 0; x < 256; x++ {
			p := img.Pix[x*4 : x*4+4]
			if p[0] != p[3] || p[1] != p[3] || p[2] != p[3] {
				t.Fatalf("%d bpp: pixel %d is not premultiplied white: %v", bpp, x, p)
			}
			seen[p[3]] = true
		}
		if len(seen) != 1<<bpp {
			t.Errorf("%d bpp: %d distinct levels, want %d", bpp, len(seen), 1<<bpp)
		}
		if !seen[0] || !seen[255] {
			t.Errorf("%d bpp: levels must include 0 and 255", bpp)
		}
	}
}

func TestQuantizeRejectsBadInput(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	for _, bpp := range []int{0, 3, 16} {
		if quantize(img, bpp, DefaultThreshold) == nil {
			t.Errorf("bpp %d accepted", bpp)
		}
	}
	for _, th := range []float64{0, -0.5, 1.01} {
		if quantize(img, 1, th) == nil {
			t.Errorf("threshold %v accepted", th)
		}
	}
}

func TestCIdentifier(t *testing.T) {
	cases := map[string]string{
		"verdanab-12":         "verdanab_12",
		"Face-68-stroke1p25":  "Face_68_stroke1p25",
		"12px":                "_12px",
		"a.b c":               "a_b_c",
		"":                    "_",
		"verdanab-12-1bpp":    "verdanab_12_1bpp",
		"MP_12pxNominal_14px": "MP_12pxNominal_14px",
		"ünï":                 "_n_",
	}
	for in, want := range cases {
		if got := CIdentifier(in); got != want {
			t.Errorf("CIdentifier(%q) = %q, want %q", in, got, want)
		}
	}
}

// cFontData is a C font source parsed back: the header fields, the glyph table and
// the bitmap bytes, laid out as the struct is in memory.
type cFontData struct {
	bpp, first, last, height int
	widths, offsets          []int
	blob                     []byte // header + glyph table + bitmaps
}

var (
	reHeaderField = regexp.MustCompile(`\.(bpp|firstChar|lastChar|height) = (0x[0-9A-F]+|\d+),`)
	reGlyph       = regexp.MustCompile(`^\t\t\{\s*(\d+), 0x([0-9A-F]{2}), 0x([0-9A-F]{4}) \},`)
	reByte        = regexp.MustCompile(`0x([0-9A-F]{2}),`)
)

func parseCFont(t *testing.T, path string) cFontData {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f cFontData
	for _, m := range reHeaderField.FindAllStringSubmatch(string(src), -1) {
		v, err := strconv.ParseInt(m[2], 0, 32)
		if err != nil {
			t.Fatal(err)
		}
		switch m[1] {
		case "bpp":
			f.bpp = 1 << v
		case "firstChar":
			f.first = int(v)
		case "lastChar":
			f.last = int(v)
		case "height":
			f.height = int(v)
		}
	}
	section := ""
	var bitmaps []byte
	for _, line := range strings.Split(string(src), "\n") {
		switch strings.TrimSpace(line) {
		case ".glyphs =":
			section = "glyphs"
			continue
		case ".bitmaps =":
			section = "bitmaps"
			continue
		}
		code := line
		if i := strings.Index(code, "//"); i >= 0 {
			code = code[:i]
		}
		switch section {
		case "glyphs":
			if m := reGlyph.FindStringSubmatch(line); m != nil {
				w, _ := strconv.Atoi(m[1])
				lsb, _ := strconv.ParseInt(m[2], 16, 32)
				msb, _ := strconv.ParseInt(m[3], 16, 32)
				f.widths = append(f.widths, w)
				f.offsets = append(f.offsets, int(msb<<8|lsb))
			}
		case "bitmaps":
			for _, m := range reByte.FindAllStringSubmatch(code, -1) {
				b, _ := strconv.ParseUint(m[1], 16, 8)
				bitmaps = append(bitmaps, byte(b))
			}
		}
	}
	f.blob = make([]byte, cHeaderSize+cGlyphSize*len(f.widths))
	f.blob = append(f.blob, bitmaps...)
	return f
}

// pixel decodes one pixel of char from the font, the way the Microchip Graphics Library's OutChar reads it:
// glyph rows of ceil(width*bpp/8) bytes, pixels packed from the least significant bit.
func (f cFontData) pixel(char rune, x, y int) int {
	i := int(char) - f.first
	stride := (f.widths[i]*f.bpp + 7) / 8
	bit := x * f.bpp
	b := f.blob[f.offsets[i]+y*stride+bit/8]
	return int(b>>(bit%8)) & (1<<f.bpp - 1)
}

func TestCSourceMatchesAtlas(t *testing.T) {
	fontPath := fontOrSkip(t)
	const chars = "A\\gA%"
	for _, bpp := range []int{1, 2, 4, 8} {
		dir := t.TempDir()
		png := filepath.Join(dir, "atlas")
		src := filepath.Join(dir, "font")
		opts := Options{Format: "png", Padding: 0, Hinting: "full", BPP: bpp, Threshold: DefaultThreshold}
		if err := GenerateWithOptions(fontPath, 20, chars, png, opts); err != nil {
			t.Fatal(err)
		}
		opts.Format = "c"
		if err := GenerateWithOptions(fontPath, 20, chars, src, opts); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(src + ".h"); err != nil {
			t.Fatalf("%d bpp: header missing: %v", bpp, err)
		}

		f := parseCFont(t, src+".c")
		if f.bpp != bpp || f.first != '%' || f.last != 'g' {
			t.Fatalf("%d bpp: header bpp=%d first=%#x last=%#x", bpp, f.bpp, f.first, f.last)
		}
		if len(f.widths) != f.last-f.first+1 {
			t.Fatalf("%d bpp: %d glyph entries for range %#x-%#x", bpp, len(f.widths), f.first, f.last)
		}
		wantOffset := cHeaderSize + cGlyphSize*len(f.widths)
		for i, w := range f.widths {
			if f.offsets[i] != wantOffset {
				t.Fatalf("%d bpp: glyph %#x at offset %d, want %d", bpp, f.first+i, f.offsets[i], wantOffset)
			}
			wantOffset += (w*bpp + 7) / 8 * f.height
		}
		if wantOffset != len(f.blob) {
			t.Fatalf("%d bpp: %d bytes of font, glyph table accounts for %d", bpp, len(f.blob), wantOffset)
		}
		if w := f.widths['B'-f.first]; w != 0 {
			t.Errorf("%d bpp: unrequested 'B' has width %d, want 0", bpp, w)
		}

		atlas := alphaImage(t, png+".png")
		if atlas.Bounds().Dy() != f.height {
			t.Fatalf("%d bpp: atlas height %d, font height %d", bpp, atlas.Bounds().Dy(), f.height)
		}
		// The atlas is in chars order, without padding; a repeated char is drawn twice.
		x0 := 0
		maxLevel := 1<<bpp - 1
		for _, r := range chars {
			w := f.widths[int(r)-f.first]
			for y := 0; y < f.height; y++ {
				for x := 0; x < w; x++ {
					want := int(atlas.AlphaAt(x0+x, y).A) * maxLevel / 255
					if got := f.pixel(r, x, y); got != want {
						t.Fatalf("%d bpp: %q pixel (%d,%d) = %d, atlas has %d", bpp, r, x, y, got, want)
					}
				}
			}
			x0 += w
		}
	}
}

func alphaImage(t *testing.T, path string) *image.Alpha {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	out := image.NewAlpha(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			out.Pix[out.PixOffset(x, y)] = uint8(a >> 8)
		}
	}
	return out
}

func TestOneBitHasNoAntialiasing(t *testing.T) {
	fontPath := fontOrSkip(t)
	out := filepath.Join(t.TempDir(), "x")
	opts := Options{Format: "png", Padding: 2, Hinting: "full", BPP: 1, Threshold: DefaultThreshold}
	if err := GenerateWithOptions(fontPath, 16, "AgO8", out, opts); err != nil {
		t.Fatal(err)
	}
	on := 0
	for _, a := range alphaImage(t, out+".png").Pix {
		if a != 0 && a != 255 {
			t.Fatalf("1 bpp image has partial coverage %d", a)
		}
		if a == 255 {
			on++
		}
	}
	if on == 0 {
		t.Fatal("1 bpp image is blank")
	}
	fnt, err := os.ReadFile(out + ".fnt")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(fnt, []byte(" smooth=0 ")) {
		t.Errorf("1 bpp fnt should say smooth=0: %s", strings.SplitN(string(fnt), "\n", 2)[0])
	}
}

func TestThresholdThinsAndThickens(t *testing.T) {
	fontPath := fontOrSkip(t)
	count := func(th float64) int {
		out := filepath.Join(t.TempDir(), "x")
		opts := Options{Format: "png", Padding: 2, Hinting: "none", BPP: 1, Threshold: th}
		if err := GenerateWithOptions(fontPath, 16, "AgO8", out, opts); err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, a := range alphaImage(t, out+".png").Pix {
			if a != 0 {
				n++
			}
		}
		return n
	}
	low, mid, high := count(0.1), count(0.5), count(0.9)
	if low <= mid || mid <= high {
		t.Errorf("pixels on at threshold 0.1/0.5/0.9 = %d/%d/%d, want strictly decreasing", low, mid, high)
	}
}

func TestDefaultOutputUnchanged(t *testing.T) {
	fontPath := fontOrSkip(t)
	a, b := filepath.Join(t.TempDir(), "x"), filepath.Join(t.TempDir(), "x")
	if err := Generate(fontPath, 24, "ABCgo8", a, "png", 2, "full", 0); err != nil {
		t.Fatal(err)
	}
	opts := Options{Format: "png", Padding: 2, Hinting: "full", BPP: 8, Threshold: 0.3}
	if err := GenerateWithOptions(fontPath, 24, "ABCgo8", b, opts); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".png", ".fnt"} {
		x, _ := os.ReadFile(a + ext)
		y, _ := os.ReadFile(b + ext)
		if !bytes.Equal(x, y) {
			t.Errorf("8 bpp %s differs from the default output", ext)
		}
	}
}

func TestCSourceRejectsWideCodes(t *testing.T) {
	fontPath := fontOrSkip(t)
	opts := Options{Format: "c", Hinting: "full", BPP: 1, Threshold: DefaultThreshold}
	err := GenerateWithOptions(fontPath, 12, "AĀ", filepath.Join(t.TempDir(), "x"), opts)
	if err == nil || !strings.Contains(err.Error(), "U+0100") {
		t.Errorf("want an error naming U+0100, got %v", err)
	}
}

func TestCharLabelNeverEndsInBackslash(t *testing.T) {
	for r := rune(0); r <= 0xFF; r++ {
		for _, present := range []bool{true, false} {
			if l := charLabel(r, present); strings.HasSuffix(l, "\\") {
				t.Errorf("charLabel(%#x) = %q ends in a backslash", r, l)
			}
		}
	}
}
