package crender

import (
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"ttf2bmp/converter"
)

// chars leaves 'B' out of a contiguous range, so the C font has a width-0 entry there.
const chars = "Ag é"

func requireTools(t *testing.T) {
	t.Helper()
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	for _, tool := range []string{"bash", cc} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not found: %v", tool, err)
		}
	}
}

func writeFont(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "goregular.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func generate(t *testing.T, ttf, prefix, format string, bpp int) {
	t.Helper()
	err := converter.GenerateWithOptions(ttf, 24, chars, prefix, converter.Options{
		Format:    format,
		Padding:   0,
		Hinting:   "full",
		BPP:       bpp,
		Threshold: converter.DefaultThreshold,
		Tight:     true,
	})
	if err != nil {
		t.Fatalf("generate %s at %d bpp: %v", format, bpp, err)
	}
}

func render(fontC, text, out string) (string, error) {
	cmd := exec.Command("bash", filepath.Join("..", "..", "scripts", "render-c-font.sh"), fontC, text, out)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// readBMP reads the renderer's top-down 24-bit grey BMP as one level per pixel.
func readBMP(t *testing.T, path string) (width, height int, pix []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 54 || string(data[:2]) != "BM" || binary.LittleEndian.Uint16(data[28:]) != 24 {
		t.Fatalf("%s: not a 24-bit BMP", path)
	}
	width = int(int32(binary.LittleEndian.Uint32(data[18:])))
	height = -int(int32(binary.LittleEndian.Uint32(data[22:])))
	if width <= 0 || height <= 0 {
		t.Fatalf("%s: want a top-down BMP, got %dx%d", path, width, -height)
	}
	stride := (width*3 + 3) &^ 3
	offset := int(binary.LittleEndian.Uint32(data[10:]))
	if len(data) < offset+stride*height {
		t.Fatalf("%s: truncated", path)
	}
	pix = make([]byte, width*height)
	for y := range height {
		for x := range width {
			p := data[offset+y*stride+3*x:]
			if p[0] != p[1] || p[1] != p[2] {
				t.Fatalf("%s: pixel (%d,%d) is not grey", path, x, y)
			}
			pix[y*width+x] = p[0]
		}
	}
	return width, height, pix
}

func readAtlasAlpha(t *testing.T, path string) (width, height int, pix []byte) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	pix = make([]byte, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			pix[(y-b.Min.Y)*b.Dx()+x-b.Min.X] = byte(a >> 8)
		}
	}
	return b.Dx(), b.Dy(), pix
}

// TestRenderMatchesAtlas draws the font's characters, in order, through the C text
// primitives and checks the result against the unpadded atlas of the same font.
func TestRenderMatchesAtlas(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	ttf := writeFont(t, dir)
	for _, bpp := range []int{1, 2, 4, 8} {
		t.Run(fmt.Sprintf("%dbpp", bpp), func(t *testing.T) {
			prefix := filepath.Join(dir, fmt.Sprintf("font%d", bpp))
			generate(t, ttf, prefix, "c", bpp)
			generate(t, ttf, prefix, "png", bpp)

			out := prefix + ".bmp"
			if msg, err := render(prefix+".c", chars, out); err != nil {
				t.Fatalf("render: %v\n%s", err, msg)
			}
			w, h, got := readBMP(t, out)
			aw, ah, want := readAtlasAlpha(t, prefix+".png")
			if w != aw || h != ah {
				t.Fatalf("size %dx%d, atlas %dx%d", w, h, aw, ah)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("pixel (%d,%d) = %d, atlas %d", i%w, i/w, got[i], want[i])
				}
			}

			// 'B' is in range with width 0: it draws nothing and does not move the cursor.
			gap := prefix + "-gap.bmp"
			if msg, err := render(prefix+".c", "AB"+chars[1:], gap); err != nil {
				t.Fatalf("render with gap: %v\n%s", err, msg)
			}
			gw, gh, gotGap := readBMP(t, gap)
			if gw != w || gh != h || string(gotGap) != string(got) {
				t.Fatalf("width-0 glyph changed the rendering")
			}
		})
	}
}

func TestRenderErrors(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	ttf := writeFont(t, dir)
	prefix := filepath.Join(dir, "font")
	generate(t, ttf, prefix, "c", 1)

	tests := []struct {
		name, text, want string
	}{
		{"above range", "Aê", "outside the font's range"},
		{"above Latin-1", "A€", "above U+00FF"},
		{"control", "A\nA", "control character"},
		{"invalid UTF-8", "A\xc3", "invalid UTF-8"},
		{"stray continuation", "A\x80", "invalid UTF-8"},
		{"overlong UTF-8", "\xc1\x81", "invalid UTF-8"},
		{"empty", "", "is empty"},
		{"zero width", "B", "is empty"},
		{"too long", strings.Repeat("A", 129), "longer than 128"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(dir, "out.bmp")
			msg, err := render(prefix+".c", tt.text, out)
			if err == nil {
				t.Fatalf("want an error, got success:\n%s", msg)
			}
			if !strings.Contains(msg, tt.want) {
				t.Fatalf("want %q in output, got:\n%s", tt.want, msg)
			}
		})
	}

	if _, err := render(prefix+".c", strings.Repeat("A", 128), filepath.Join(dir, "max.bmp")); err != nil {
		t.Fatalf("128 characters should render: %v", err)
	}
}
