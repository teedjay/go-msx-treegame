package manifest

// Entries is the asset manifest, in extraction order (a Derived entry must
// follow its source). Names give the ROM address and the VRAM table the
// asset is written to; a role is added only once it is confirmed.
var Entries = []Entry{
	{
		Name: "font_45eb", Kind: Raw, Addr: 0x45EB, Len: 0x180,
		Confidence: "High: copied by $44B5-$44C5 (LD HL,$45EB / LD BC,$0180 / JP $446E) to the " +
			"pattern table at DE+$2000. The last $28 bytes overlap the text strings at $4743; the " +
			"original copies them into VRAM regardless, so the asset keeps the full $180.",
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
