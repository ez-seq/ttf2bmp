package main

import (
	"math"
	"testing"
)

func TestStrokeSuffix(t *testing.T) {
	cases := map[float64]string{0: "", 1: "-stroke1", 0.77: "-stroke0p77", 1.25: "-stroke1p25", 2: "-stroke2"}
	for w, want := range cases {
		if got := strokeSuffix(w); got != want {
			t.Errorf("strokeSuffix(%v) = %q, want %q", w, got, want)
		}
	}
}

func TestValidateStroke(t *testing.T) {
	for _, w := range []float64{0, 0.125, 0.5, 1, 3.25} {
		cfg, err := validateInputs("f.ttf", "12", "A", ".", "png", 2, "none", w, 0, 0.5)
		if err != nil || cfg.Stroke != w {
			t.Errorf("stroke %v: got %v, %v", w, cfg.Stroke, err)
		}
	}
	for _, w := range []float64{-1, 0.01, 0.1, math.NaN(), math.Inf(1)} {
		if _, err := validateInputs("f.ttf", "12", "A", ".", "png", 2, "none", w, 0, 0.5); err == nil {
			t.Errorf("stroke %v: accepted, want an error", w)
		}
	}
}

func TestBPPSuffix(t *testing.T) {
	cases := map[int]string{8: "", 1: "-1bpp", 2: "-2bpp", 4: "-4bpp"}
	for b, want := range cases {
		if got := bppSuffix(b); got != want {
			t.Errorf("bppSuffix(%d) = %q, want %q", b, got, want)
		}
	}
}

func TestValidateBPP(t *testing.T) {
	defaults := map[string]int{"png": 8, "bmp": 8, "c": 1}
	for typ, want := range defaults {
		cfg, err := validateInputs("f.ttf", "12", "A", ".", typ, 2, "full", 0, 0, 0.5)
		if err != nil || cfg.BPP != want {
			t.Errorf("type %s default bpp: got %d, %v; want %d", typ, cfg.BPP, err, want)
		}
	}
	for _, b := range []int{1, 2, 4, 8} {
		cfg, err := validateInputs("f.ttf", "12", "A", ".", "c", 2, "full", 0, b, 0.5)
		if err != nil || cfg.BPP != b {
			t.Errorf("bpp %d: got %d, %v", b, cfg.BPP, err)
		}
	}
	for _, b := range []int{-1, 3, 16} {
		if _, err := validateInputs("f.ttf", "12", "A", ".", "png", 2, "full", 0, b, 0.5); err == nil {
			t.Errorf("bpp %d: accepted, want an error", b)
		}
	}
	for _, a := range []float64{0, -0.1, 1.5, math.NaN()} {
		if _, err := validateInputs("f.ttf", "12", "A", ".", "png", 2, "full", 0, 1, a); err == nil {
			t.Errorf("threshold %v: accepted, want an error", a)
		}
	}
}
