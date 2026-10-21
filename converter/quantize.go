package converter

import (
	"fmt"
	"image"
)

// DefaultThreshold is the coverage at or above which a pixel is on at 1 bit per pixel.
const DefaultThreshold = 0.5

// ValidBPP reports whether bpp is a supported colour depth: 1, 2, 4 or 8 bits per
// pixel, the depths the Microchip GFX font header can describe.
func ValidBPP(bpp int) bool {
	return bpp == 1 || bpp == 2 || bpp == 4 || bpp == 8
}

// ValidThreshold reports whether t is a usable 1-bit coverage cutoff: above 0, so that
// blank pixels stay off, and at most 1. NaN is refused too.
func ValidThreshold(t float64) bool {
	return t > 0 && t <= 1
}

// level maps an 8-bit coverage to one of the 2^bpp levels of the given depth. At 1 bpp
// the pixel is on when its coverage reaches threshold; deeper levels round to nearest.
func level(a uint8, bpp int, threshold float64) uint8 {
	if bpp == 1 {
		if float64(a) >= threshold*255 {
			return 1
		}
		return 0
	}
	maxLevel := 1<<bpp - 1
	return uint8((int(a)*maxLevel + 127) / 255)
}

// quantize reduces the coverage of every pixel of img to 2^bpp levels, and writes it
// back on the 0-255 scale, so the image shows exactly what the target screen will.
// 8 bpp leaves img untouched. Generate draws white, so the colour channels equal the
// premultiplied alpha and are set with it.
func quantize(img *image.RGBA, bpp int, threshold float64) error {
	if !ValidBPP(bpp) {
		return fmt.Errorf("bpp %d: must be 1, 2, 4 or 8", bpp)
	}
	if bpp == 1 && !ValidThreshold(threshold) {
		return fmt.Errorf("threshold %v: must be above 0 and at most 1", threshold)
	}
	if bpp == 8 {
		return nil
	}
	maxLevel := 1<<bpp - 1
	for i := 0; i+3 < len(img.Pix); i += 4 {
		v := uint8(int(level(img.Pix[i+3], bpp, threshold)) * 255 / maxLevel)
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, v, v, v
	}
	return nil
}
