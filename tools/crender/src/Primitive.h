/*
 * Host build of the Microchip Graphics Library (MLA) text primitives, for testing fonts
 * exported by ttf2bmp. The types, names and macros follow MLA's Primitive.h, so an
 * exported font compiles here unchanged and Primitive.c reads side by side with an
 * MLA-derived target's own.
 */
#ifndef TTF2BMP_PRIMITIVE_H
#define TTF2BMP_PRIMITIVE_H

/* The font handle exported fonts point to: the font's address only. MLA's own handle,
 * FONT_FLASH, carries a memory type before the address. */
typedef struct
{
	const char * address;
} SFontHandle;

typedef struct
{
	unsigned char  fontID;
	unsigned char  extendedGlyphEntry : 1;
	unsigned char  res1               : 1;
	unsigned char  bpp                : 2;	/* actual bpp = 2^bpp */
	unsigned char  orientation        : 2;
	unsigned char  res2               : 2;
	unsigned short firstChar;
	unsigned short lastChar;
	unsigned short height;
} FONT_HEADER;

typedef struct
{
	unsigned char  width;
	unsigned char  offsetLSB;
	unsigned short offsetMSB;
} GLYPH_ENTRY;

_Static_assert(sizeof(FONT_HEADER) == 8, "FONT_HEADER layout");
_Static_assert(sizeof(GLYPH_ENTRY) == 4, "GLYPH_ENTRY layout");

typedef struct
{
	void        *pFont;
	FONT_HEADER  fontHeader;
} GFX_FONT_CURRENT;

typedef struct
{
	const GLYPH_ENTRY   *pChTable;
	unsigned char        bpp;
	short                chGlyphWidth;
	const unsigned char *pChImage;
	short                xAdjust;
	short                yAdjust;
	short                xWidthAdjust;
	short                heightOvershoot;
} OUTCHAR_PARAM;

extern short _cursorX;
extern short _cursorY;

#define GetX()  _cursorX
#define GetY()  _cursorY
#define MoveTo(x, y) \
	_cursorX = x;    \
	_cursorY = y;

void           SetFont(void *pFont);
unsigned short OutChar(const unsigned char iChar);
unsigned short OutText(const char *textString);
unsigned short OutTextXY(short x, short y, const char *textString);
short          GetTextWidth(const char *textString, void *pFont);
short          GetTextHeight(void *pFont);

/* Display driver, as on the target: draws one byte of a 1 bpp glyph row, least
 * significant bit leftmost, from iColumnStart up to (not including) iColumnEnd. */
void PutPixelMask(const unsigned char iBitMask,
				  const unsigned int iColumnStart,
				  const unsigned int iColumnEnd,
				  const unsigned int iRow);

/* Host-only extension of PutPixelMask for 2, 4 and 8 bpp glyph rows: draws the
 * 8 / iBpp pixels packed in one byte, iBpp bits each, least significant first. */
void PutPixelLevels(const unsigned char iByte,
					const unsigned int iBpp,
					const unsigned int iColumnStart,
					const unsigned int iColumnEnd,
					const unsigned int iRow);

#endif
