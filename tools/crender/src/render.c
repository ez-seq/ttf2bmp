/*
 * Renders a line of text with a font exported by ttf2bmp (--type c), through the MLA
 * text primitives in Primitive.c, and writes it as a 24-bit BMP, white on black.
 *
 * Build with the font source and -DFONT_HANDLE=<gpOpaqueFont_... symbol>; see
 * scripts/render-c-font.sh. Usage: render TEXT OUTPUT.bmp
 *
 * TEXT is UTF-8 and limited to Latin-1, the range MLA fonts index by byte.
 */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "Primitive.h"

#ifndef FONT_HANDLE
#error "define FONT_HANDLE as the font's gpOpaqueFont_... symbol"
#endif

extern void * FONT_HANDLE;

/* Keeps the GetTextWidth sum (a short) from overflowing: 128 glyphs of width <= 255. */
#define MAX_TEXT 129

/* Display driver: an 8-bit grayscale framebuffer standing in for the target's. */

static unsigned char * gFrame;
static unsigned int gFrameWidth;
static unsigned int gFrameHeight;

static void PutPixelLevel(unsigned int iColumn, unsigned int iRow, unsigned char iLevel)
{
	if((iColumn >= gFrameWidth) || (iRow >= gFrameHeight))
		return;
	gFrame[(size_t)iRow * gFrameWidth + iColumn] = iLevel;
}

void PutPixelMask(const unsigned char iBitMask,
				  const unsigned int iColumnStart,
				  const unsigned int iColumnEnd,
				  const unsigned int iRow)
{
	unsigned int colIndex;
	unsigned char curBitMask = iBitMask;

	for(colIndex = iColumnStart; colIndex < iColumnEnd; colIndex++, curBitMask = (curBitMask >> 1))
	{
		if(curBitMask & 0x1)
			PutPixelLevel(colIndex, iRow, 0xFF);
	}
}

void PutPixelLevels(const unsigned char iByte,
					const unsigned int iBpp,
					const unsigned int iColumnStart,
					const unsigned int iColumnEnd,
					const unsigned int iRow)
{
	const unsigned int kMax = (1u << iBpp) - 1;
	unsigned int colIndex;
	unsigned int curByte = iByte;
	unsigned int level;

	for(colIndex = iColumnStart; (colIndex < iColumnEnd) && (colIndex < iColumnStart + 8 / iBpp);
		colIndex++, curByte >>= iBpp)
	{
		level = curByte & kMax;
		if(level)
			PutPixelLevel(colIndex, iRow, (unsigned char)((level * 0xFF + kMax / 2) / kMax));
	}
}

/* Command line */

static int decodeLatin1(const char * iUTF8, char * oText, size_t iCap)
{
	const unsigned char * p = (const unsigned char *) iUTF8;
	size_t n = 0;
	unsigned int code;

	while(*p)
	{
		if(*p < 0x80)
		{
			code = *p++;
		}
		else if(((p[0] & 0xE0) == 0xC0) && ((p[1] & 0xC0) == 0x80))
		{
			code = ((unsigned int)(p[0] & 0x1F) << 6) | (p[1] & 0x3F);
			if(code < 0x80)
			{
				fprintf(stderr, "error: invalid UTF-8 in text\n");
				return -1;
			}
			p += 2;
		}
		else if((p[0] >= 0xE0) && (p[0] <= 0xF4) && ((p[1] & 0xC0) == 0x80) && ((p[2] & 0xC0) == 0x80))
		{
			fprintf(stderr, "error: text has characters above U+00FF, which MLA fonts cannot index\n");
			return -1;
		}
		else
		{
			fprintf(stderr, "error: invalid UTF-8 in text\n");
			return -1;
		}
		if(code < 0x20)
		{
			fprintf(stderr, "error: text has control character 0x%02X; only one line is rendered\n", code);
			return -1;
		}
		if(n + 1 >= iCap)
		{
			fprintf(stderr, "error: text is longer than %d characters\n", MAX_TEXT - 1);
			return -1;
		}
		oText[n++] = (char) code;
	}
	oText[n] = '\0';
	return 0;
}

static void putLE16(unsigned char * o, uint16_t v)
{
	o[0] = (unsigned char) v;
	o[1] = (unsigned char)(v >> 8);
}

static void putLE32(unsigned char * o, uint32_t v)
{
	putLE16(o, (uint16_t) v);
	putLE16(o + 2, (uint16_t)(v >> 16));
}

/* Writes the framebuffer as a top-down 24-bit BMP. */
static int writeBMP(const char * iPath)
{
	const uint32_t kStride = (gFrameWidth * 3 + 3) & ~3u;
	const uint32_t kImageSize = kStride * gFrameHeight;
	unsigned char header[54] = {'B', 'M'};
	unsigned char * row;
	unsigned int x;
	unsigned int y;
	FILE * f;
	int ok;

	putLE32(header + 2, 54 + kImageSize);
	putLE32(header + 10, 54);
	putLE32(header + 14, 40);
	putLE32(header + 18, gFrameWidth);
	putLE32(header + 22, (uint32_t)(-(int32_t) gFrameHeight));
	putLE16(header + 26, 1);
	putLE16(header + 28, 24);
	putLE32(header + 34, kImageSize);
	putLE32(header + 38, 2835);
	putLE32(header + 42, 2835);

	row = calloc(kStride, 1);
	f = fopen(iPath, "wb");
	if(!row || !f)
	{
		perror(iPath);
		free(row);
		if(f)
			fclose(f);
		return -1;
	}
	ok = fwrite(header, sizeof(header), 1, f) == 1;
	for(y = 0; ok && (y < gFrameHeight); y++)
	{
		for(x = 0; x < gFrameWidth; x++)
			memset(row + 3 * x, gFrame[(size_t)y * gFrameWidth + x], 3);
		ok = fwrite(row, kStride, 1, f) == 1;
	}
	free(row);
	if((fclose(f) != 0) || !ok)
	{
		perror(iPath);
		return -1;
	}
	return 0;
}

int main(int argc, char ** argv)
{
	static char text[MAX_TEXT];
	const FONT_HEADER * pHeader;
	const unsigned char * p;
	short width;
	short height;

	if(argc != 3)
	{
		fprintf(stderr, "usage: %s TEXT OUTPUT.bmp\n", argv[0]);
		return 2;
	}
	if(decodeLatin1(argv[1], text, sizeof(text)) != 0)
		return 1;

	pHeader = (const FONT_HEADER *)((SFontHandle *) FONT_HANDLE)->address;
	if(pHeader->extendedGlyphEntry)
	{
		fprintf(stderr, "error: extended glyph entries are not supported\n");
		return 1;
	}
	/* The target's OutChar skips characters outside the font; flag them instead. */
	for(p = (const unsigned char *) text; *p; p++)
	{
		if((*p < pHeader->firstChar) || (*p > pHeader->lastChar))
		{
			fprintf(stderr, "error: U+%04X is outside the font's range U+%04X-U+%04X\n",
					*p, pHeader->firstChar, pHeader->lastChar);
			return 1;
		}
	}

	SetFont(FONT_HANDLE);
	width = GetTextWidth(text, FONT_HANDLE);
	height = GetTextHeight(FONT_HANDLE);
	if((width <= 0) || (height <= 0))
	{
		fprintf(stderr, "error: rendered size %dx%d is empty\n", width, height);
		return 1;
	}

	gFrameWidth = (unsigned int) width;
	gFrameHeight = (unsigned int) height;
	gFrame = calloc((size_t) gFrameWidth * gFrameHeight, 1);
	if(!gFrame)
	{
		perror("calloc");
		return 1;
	}
	OutTextXY(0, 0, text);
	if(GetX() != width)
	{
		fprintf(stderr, "error: cursor ended at x=%d, expected %d\n", GetX(), width);
		free(gFrame);
		return 1;
	}

	if(writeBMP(argv[2]) != 0)
	{
		free(gFrame);
		return 1;
	}
	free(gFrame);
	return 0;
}
