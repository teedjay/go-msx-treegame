package manifest

// Entries is the asset manifest, in extraction order (a Derived entry must
// follow its source). Names give the ROM address and the VRAM table the
// asset is written to; a role is added only once it is confirmed.
var Entries = []Entry{
	{
		Name: "txt_426d", Kind: Raw, Addr: 0x426D, Len: 16,
		Confidence: "Medium: text drawn by $41CA via $4079; fills the gap $426D-$427C exactly.",
	},
	{
		Name: "tab_45e7", Kind: Raw, Addr: 0x45E7, Len: 4,
		Confidence: "High: $45B9-$45C3 indexes it with the menu selection RAM[$042] (0-3) and " +
			"stores the byte to RAM[$002] (game mode). Ends exactly where the font starts.",
	},
	{
		Name: "font_45eb", Kind: Raw, Addr: 0x45EB, Len: 0x180,
		Confidence: "High: copied by $44B5-$44C5 (LD HL,$45EB / LD BC,$0180 / JP $446E) to the " +
			"pattern table at DE+$2000. The last $28 bytes overlap the text strings at $4743; the " +
			"original copies them into VRAM regardless, so the asset keeps the full $180.",
	},
	{
		Name: "txt_4743", Kind: Raw, Addr: 0x4743, Len: 0x47EF - 0x4743,
		Confidence: "Medium: text strings for $4079 (address, chars, $FE new address, $FF end), " +
			"referenced at $4743, $4752, $4758, $475E, $47D7 and $47E4; ends where name_47ef starts.",
	},
	{
		Name: "name_47ef", Kind: RLE, Addr: 0x47EF, End: 0x4800,
		Segments:   []Segment{{0x394A, 12}, {0x396C, 8}},
		Confidence: "High for extent: loaded via $44C8 at $4118; ends exactly where $4800 starts.",
	},
	{
		Name: "pat_4800", Kind: RLE, Addr: 0x4800, End: 0x48C2,
		Segments:   []Segment{{0x2600, 256}},
		Confidence: "High for extent: loaded via $44C8 at $4292; ends exactly where $48C2 starts.",
	},
	{
		Name: "col_48c2", Kind: RLEAt, Addr: 0x48C2, Len: 16, End: 0x48CD,
		Confidence: "High for extent: loaded via $44CC at $42A2 in a loop with caller-supplied DE; " +
			"ends exactly at code $48CD.",
	},
	{
		Name: "pat_4923", Kind: RLEAt, Addr: 0x4923, Len: 216, End: 0x49BA,
		Confidence: "High: loaded via $44CC at $48E1 with DE=$2300; ends exactly at code $49BA.",
	},
	{
		Name: "pat_5473", Kind: RLE, Addr: 0x5473, End: 0x5968,
		Segments:   []Segment{{0x2038, 328}, {0x2300, 1272}},
		Confidence: "High: loaded via $44C8 at $60FD; ends exactly where $5968 starts.",
	},
	{
		Name: "col_5968", Kind: RLE, Addr: 0x5968, End: 0x5BFE,
		Segments: []Segment{{0x0008, 376}, {0x0300, 1272}},
		Confidence: "High: loaded at $6103 via $615D, which substitutes colour nibble 3 with the " +
			"matching nibble of RAM[$05D] as it writes. The asset is the unsubstituted decode; " +
			"segments mirror pat_5473 exactly.",
	},
	{
		Name: "spr_5bfe", Kind: Raw, Addr: 0x5BFE, Len: 0x140,
		Confidence: "High: copied by $6109-$6112 (DE=$1800, HL=$5BFE, BC=$0140) to the sprite " +
			"pattern table; $5BFE+$140 is exactly the next stream at $5D3E.",
	},
	{
		Name: "spr_5bfe_mirror", Kind: Derived, Source: "spr_5bfe", Derive: mirrorSprites,
		Confidence: "High: built by $6115-$6148 into RAM $E0B0 and written to VRAM $1940 by $614A.",
	},
	{
		Name: "spr_5d3e", Kind: RLE, Addr: 0x5D3E, End: 0x60FA,
		Segments:   []Segment{{0x1B00, 1248}},
		Confidence: "High: loaded via $44C8 at $6159; ends exactly at the loader code $60FA.",
	},
	{
		Name: "tab_745a", Kind: Raw, Addr: 0x745A, Len: 8,
		Confidence: "High: $7431-$743B indexes it with B&7 and stores the byte to RAM[$05D], " +
			"the colour that $615D substitutes for nibble 3. Fills the 8-byte gap $745A-$7461.",
	},
	{
		Name: "data_4a7e", Kind: Raw, Addr: 0x4A7E, Len: 0x73E,
		Confidence: "Medium: object shape and stage map tables; pointer tables at $515E/$5174/$519E/$51B6 read by $4A3C, stage lists at $4E49 (read by $52FD) and $4D5E (by $5317). Fills the gap $4A7E-$51BB exactly.",
	},
	{
		Name: "data_51ef", Kind: Raw, Addr: 0x51EF, Len: 0x11,
		Confidence: "High: copied to RAM $E050 by $51D6-$51DF (LD BC,$0011): new-game player values. Fills the gap exactly.",
	},
	{
		Name: "data_53c2", Kind: Raw, Addr: 0x53C2, Len: 0xE,
		Confidence: "High: $5377 copies 4 bytes 4 times ($53C2), $5366 copies 5 ($53C6), $535B reads 5 ($53CB): 4+5+5 fills the gap exactly.",
	},
	{
		Name: "demo_5412", Kind: Raw, Addr: 0x5412, Len: 0x61,
		Confidence: "High: demo input script, (duration, input) pairs read by $53D0 from the pointer set at $5405; ends where pat_5473 starts.",
	},
	{
		Name: "data_620f", Kind: Raw, Addr: 0x620F, Len: 0x6,
		Confidence: "Low: referenced by $62AE; fills the gap $620F-$6214.",
	},
	{
		Name: "data_65a2", Kind: Raw, Addr: 0x65A2, Len: 0x13,
		Confidence: "Medium: tables at $65A3 and $65A7 (refs $630C, $6AE4, $713D, ...); fills the gap $65A2-$65B4.",
	},
	{
		Name: "data_6630", Kind: Raw, Addr: 0x6630, Len: 0x4D,
		Confidence: "Medium: player sprite layout tables at $6630 (read by $65DC) and $6669 (by $6611); fills the gap $6630-$667C.",
	},
	{
		Name: "txt_6718", Kind: Raw, Addr: 0x6718, Len: 0xE,
		Confidence: "High: text table for $4079 (the HEIGHT label), drawn by $66ED; fills the gap.",
	},
	{
		Name: "data_6a73", Kind: Raw, Addr: 0x6A73, Len: 0xD,
		Confidence: "High: 4-byte records ending in $FF, searched by $6A5C-$6A71; fills the gap.",
	},
	{
		Name: "data_6bec", Kind: Raw, Addr: 0x6BEC, Len: 0x4,
		Confidence: "Low: referenced by $6BB5; fills the gap.",
	},
	{
		Name: "data_6fdc", Kind: Raw, Addr: 0x6FDC, Len: 0xC,
		Confidence: "Low: referenced by $6FD6; fills the gap.",
	},
	{
		Name: "data_723a", Kind: Raw, Addr: 0x723A, Len: 0x4,
		Confidence: "Low: referenced by $722E; fills the gap.",
	},
	{
		Name: "data_75a0", Kind: Raw, Addr: 0x75A0, Len: 0x18,
		Confidence: "Low: referenced by $74D4 and others; ends where name_75b8 starts.",
	},
	{
		Name: "data_75d0", Kind: Raw, Addr: 0x75D0, Len: 0x7D,
		Confidence: "Low: referenced by $752B, $7549, $7525, $74B3 and others; starts where name_75b8 ends, fills the gap to $764C.",
	},
	{
		Name: "data_769c", Kind: Raw, Addr: 0x769C, Len: 0x169,
		Confidence: "Low: referenced by $7422 and $7427; fills the gap $769C-$7804.",
	},
	{
		Name: "data_7a19", Kind: Raw, Addr: 0x7A19, Len: 0x10,
		Confidence: "Low: referenced by $79FD; fills the gap.",
	},
	{
		Name: "snd_7b4f", Kind: Raw, Addr: 0x7B4F, Len: 4,
		Confidence: "High: $7B42 copies these 4 bytes with LDDR to RAM $E032-$E035, a sound " +
			"sequence that $7B25 then sweeps in RAM (sound id $0B).",
	},
	{
		Name: "snd_7c89", Kind: Raw, Addr: 0x7C89, Len: 0x377,
		Confidence: "Medium: note period table at $7C89 (12 entries, read by $7C6A), sound " +
			"pointer table at $7C93 (read by $7A96), and the sequences they point to, through " +
			"the end of the ROM. Exact boundaries inside the block not yet traced.",
	},
	{
		Name: "name_75b8", Kind: RLE, Addr: 0x75B8, End: 0x75D0,
		Segments:   []Segment{{0x394A, 12}, {0x398B, 10}},
		Confidence: "Medium: loaded via $44C8 at $7543; the bytes after $75D0 are not yet classified.",
	},
}

// mirrorSprites flips 16x16 sprite patterns horizontally.
//
// Ported from $6115-$6148 (High). For each of 10 sprites and each of 16
// rows it takes the left byte (from the TL/BL half) into H' and the right
// byte (TR/BR half, 16 bytes on) into L', then 16 times shifts HL left and
// rotates the carry into the 16-bit pair (IX+$00):(IX+$10). The first bit
// out (leftmost pixel) ends up rightmost, so the new left half is the
// reversed right half and vice versa.
func mirrorSprites(src []byte) []byte {
	out := make([]byte, len(src))
	for s := 0; s+32 <= len(src); s += 32 {
		for row := 0; row < 16; row++ {
			left, right := src[s+row], src[s+16+row]
			out[s+row] = reverse(right)
			out[s+16+row] = reverse(left)
		}
	}
	return out
}

func reverse(b byte) byte {
	var r byte
	for i := 0; i < 8; i++ {
		r = r<<1 | b&1
		b >>= 1
	}
	return r
}
