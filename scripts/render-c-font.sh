#!/usr/bin/env bash
# Compile and render an exported C font without the original TTF or atlas.
set -euo pipefail

if (( $# < 3 || $# > 4 )); then
	printf 'Usage: %s FONT.c TEXT OUTPUT.bmp [HANDLE_SYMBOL]\n' "$0" >&2
	exit 1
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
font="$1"
header="${font%.c}.h"
if [[ ! -f "$font" ]]; then
	printf 'error: font source not found: %s\n' "$font" >&2
	exit 1
fi
font="$(cd "$(dirname "$font")" && pwd)/$(basename "$font")"
if (( $# == 4 )); then
	symbol="$4"
else
	if [[ ! -f "$header" ]]; then
		printf 'error: header not found: %s; supply HANDLE_SYMBOL explicitly\n' "$header" >&2
		exit 1
	fi
	symbol="$(sed -nE 's/^[[:space:]]*extern[[:space:]]+void[[:space:]]*\*[[:space:]]*(gpOpaqueFont_[A-Za-z0-9_]+)[[:space:]]*;.*/\1/p' "$header")"
fi
if [[ ! "$symbol" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]]; then
	printf 'error: expected one C handle symbol; supply HANDLE_SYMBOL explicitly\n' >&2
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -f "$tmp/render"; rmdir "$tmp"' EXIT
"${CC:-cc}" -std=c11 -Wall -Wextra -Werror -pedantic \
	-I "$root/tools/crender/src" "-DFONT_HANDLE=$symbol" \
	"$font" "$root/tools/crender/src/Primitive.c" "$root/tools/crender/src/render.c" \
	-o "$tmp/render"
"$tmp/render" "$2" "$3"
