package main

import "testing"

func TestParseFontMatch(t *testing.T) {
	family, path, err := parseFontMatch("DejaVu Sans\n/usr/share/fonts/dejavu.ttf\n")
	if err != nil {
		t.Fatal(err)
	}
	if family != "DejaVu Sans" || path != "/usr/share/fonts/dejavu.ttf" {
		t.Fatalf("parseFontMatch() = %q, %q; want DejaVu Sans and font path", family, path)
	}
}

func TestParseFontMatchRejectsIncompleteOutput(t *testing.T) {
	for _, output := range []string{"", "DejaVu Sans\n", "\n/path/font.ttf", "family\n\n"} {
		if _, _, err := parseFontMatch(output); err == nil {
			t.Errorf("parseFontMatch(%q) succeeded, want an error", output)
		}
	}
}
