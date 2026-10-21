/*
 * Host implementation of the MLA text primitives, structured like an MLA-derived
 * target's Primitive.c (SetFont, OutChar, OutText, OutTextXY, GetTextWidth,
 * GetTextHeight), so the two can be compared directly. Differences from the target:
 *   - glyphs deeper than 1 bpp are drawn, through PutPixelLevels;
 *   - extended glyph entries are not supported (ttf2bmp does not emit them).
 */
#include <string.h>

#include "Primitive.h"

short _cursorX;
short _cursorY;

static GFX_FONT_CURRENT currentFont;

void SetFont(void *pFont)
{
	memcpy(&(currentFont.fontHeader), ((SFontHandle *) pFont)->address, sizeof(FONT_HEADER));
	currentFont.pFont = pFont;
}

/* Draws iChar at the cursor and advances the cursor by its width. Returns 1, or
 * 0xFFFF for a character outside the font's range, which neither draws nor moves. */
unsigned short OutChar(const unsigned char iChar)
{
	static OUTCHAR_PARAM renderData;
	const GLYPH_ENTRY * pChTable = 0;
	const char * pFontInfo = (((SFontHandle *) currentFont.pFont)->address);
	unsigned int yStart = 0;
	unsigned int yEnd = 0;
	unsigned int y = 0;
	unsigned int xStart = 0;
	unsigned int xEnd = 0;
	unsigned int x = 0;

	renderData.xAdjust = 0;
	renderData.yAdjust = 0;
	renderData.xWidthAdjust = 0;
	renderData.heightOvershoot = 0;

	if(iChar < (unsigned char)currentFont.fontHeader.firstChar)
		return (-1);
	if(iChar > (unsigned char)currentFont.fontHeader.lastChar)
		return (-1);

	renderData.bpp = 1 << currentFont.fontHeader.bpp;
	pChTable = (const GLYPH_ENTRY *) (pFontInfo + sizeof(FONT_HEADER)) + (iChar - (unsigned char)currentFont.fontHeader.firstChar);
	renderData.pChImage = (const unsigned char *) (pFontInfo + ((unsigned int)(pChTable->offsetMSB) << 8) + pChTable->offsetLSB);
	renderData.chGlyphWidth = pChTable->width;

	yStart = GetY() + renderData.yAdjust;
	yEnd = yStart + currentFont.fontHeader.height + renderData.heightOvershoot;
	xStart = GetX() + renderData.xAdjust;
	xEnd = xStart + renderData.chGlyphWidth;
	for(y = yStart; y < yEnd; ++y)
	{
		if(renderData.bpp == 1)
		{
			for(x = xStart; x < xEnd; x += 8)
				PutPixelMask(*(renderData.pChImage)++, x, xEnd, y);
		}
		else
		{
			for(x = xStart; x < xEnd; x += 8 / renderData.bpp)
				PutPixelLevels(*(renderData.pChImage)++, renderData.bpp, x, xEnd, y);
		}
	}
	_cursorX = xEnd - renderData.xAdjust - renderData.xWidthAdjust;
	return 1;
}

/* Draws textString up to its terminator or the first control code (0x01-0x0F). */
unsigned short OutText(const char *textString)
{
	char ch;
	static unsigned short counter = 0;

	while((unsigned char)(ch = *(textString + counter)) > (unsigned char)15)
	{
		if(OutChar(ch) == 0)
			return (0);
		counter++;
	}

	counter = 0;
	return (1);
}

unsigned short OutTextXY(short x, short y, const char *textString)
{
	static unsigned char start = 1;

	if(start)
	{
		MoveTo(x, y);
		start = 0;
	}

	if(OutText(textString) == 0)
	{
		return (0);
	}
	else
	{
		start = 1;
		return (1);
	}
}

/* Sums the widths of the characters of textString in pFont's range, skipping the rest. */
short GetTextWidth(const char *textString, void *pFont)
{
	const GLYPH_ENTRY *pChTable = 0;
	const FONT_HEADER *pHeader;
	short textWidth;
	char ch;
	char fontFirstChar;
	char fontLastChar;

	pHeader = (const FONT_HEADER *) ((SFontHandle *) pFont)->address;
	fontFirstChar = pHeader->firstChar;
	fontLastChar = pHeader->lastChar;
	pChTable = (const GLYPH_ENTRY *) (pHeader + 1);
	textWidth = 0;
	while((unsigned char)15 < (unsigned char)(ch = *textString++))
	{
		if((unsigned char)ch < (unsigned char)fontFirstChar)
			continue;
		if((unsigned char)ch > (unsigned char)fontLastChar)
			continue;
		textWidth += (pChTable + ((unsigned char)ch - (unsigned char)fontFirstChar))->width;
	}

	return (textWidth);
}

short GetTextHeight(void *pFont)
{
	if(pFont == currentFont.pFont)
		return currentFont.fontHeader.height;
	return ((const FONT_HEADER *) ((SFontHandle *) pFont)->address)->height;
}
