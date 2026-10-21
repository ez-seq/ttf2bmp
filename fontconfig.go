package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func resolveFontFamily(family string) (string, error) {
	output, err := exec.Command("fc-match", "-f", "%{family[0]}\\n%{file}\\n", family).Output()
	if err != nil {
		return "", fmt.Errorf("looking up system font family %q with fc-match: %w", family, err)
	}
	resolvedFamily, path, err := parseFontMatch(string(output))
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(resolvedFamily, family) {
		return "", fmt.Errorf("font family %q is not installed (Fontconfig matched %q instead)", family, resolvedFamily)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("checking font file for family %q: %w", family, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("font file for family %q is not a regular file: %s", family, path)
	}
	return path, nil
}

func parseFontMatch(output string) (family, path string, err error) {
	fields := strings.SplitN(strings.TrimSpace(output), "\n", 2)
	if len(fields) != 2 || strings.TrimSpace(fields[0]) == "" || strings.TrimSpace(fields[1]) == "" {
		return "", "", fmt.Errorf("fc-match returned invalid font information")
	}
	return strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1]), nil
}
