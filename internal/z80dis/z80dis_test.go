package z80dis

import "testing"

func fetch(b []byte, base uint16) Fetcher {
	return func(a uint16) byte {
		i := int(a) - int(base)
		if i < 0 || i >= len(b) {
			return 0
		}
		return b[i]
	}
}

func TestDecode(t *testing.T) {
	cases := []struct {
		b    []byte
		text string
		n    int
		flow Flow
	}{
		{[]byte{0x00}, "NOP", 1, FlowNext},
		{[]byte{0x21, 0x34, 0x12}, "LD HL,$1234", 3, FlowNext},
		{[]byte{0x3A, 0x05, 0xE0}, "LD A,($E005)", 3, FlowNext},
		{[]byte{0x32, 0x9A, 0xFD}, "LD ($FD9A),A", 3, FlowNext},
		{[]byte{0x18, 0xFE}, "JR $4000", 2, FlowJump},
		{[]byte{0x10, 0xFE}, "DJNZ $4000", 2, FlowBranch},
		{[]byte{0x20, 0x02}, "JR NZ,$4004", 2, FlowBranch},
		{[]byte{0xC3, 0x8E, 0x40}, "JP $408E", 3, FlowJump},
		{[]byte{0xCA, 0x00, 0x50}, "JP Z,$5000", 3, FlowBranch},
		{[]byte{0xCD, 0x47, 0x00}, "CALL $0047", 3, FlowCall},
		{[]byte{0xD4, 0x47, 0x00}, "CALL NC,$0047", 3, FlowCall},
		{[]byte{0xC9}, "RET", 1, FlowRet},
		{[]byte{0xD0}, "RET NC", 1, FlowCondRet},
		{[]byte{0xE9}, "JP (HL)", 1, FlowIndirect},
		{[]byte{0xDD, 0xE9}, "JP (IX)", 2, FlowIndirect},
		{[]byte{0xFF}, "RST $0038", 1, FlowCall},
		{[]byte{0x76}, "HALT", 1, FlowNext},
		{[]byte{0x46}, "LD B,(HL)", 1, FlowNext},
		{[]byte{0xCB, 0xB6}, "RES 6,(HL)", 2, FlowNext},
		{[]byte{0xCB, 0x46}, "BIT 0,(HL)", 2, FlowNext},
		{[]byte{0xCB, 0x30}, "SLL B", 2, FlowNext},
		{[]byte{0xED, 0xB0}, "LDIR", 2, FlowNext},
		{[]byte{0xED, 0xB3}, "OTIR", 2, FlowNext},
		{[]byte{0xED, 0x56}, "IM 1", 2, FlowNext},
		{[]byte{0xED, 0x79}, "OUT (C),A", 2, FlowNext},
		{[]byte{0xED, 0x78}, "IN A,(C)", 2, FlowNext},
		{[]byte{0xED, 0x5F}, "LD A,R", 2, FlowNext},
		{[]byte{0xED, 0x4D}, "RETI", 2, FlowRet},
		{[]byte{0xED, 0x53, 0x00, 0xE1}, "LD ($E100),DE", 4, FlowNext},
		{[]byte{0xED, 0x00}, "DB $ED,$00", 2, FlowNext},
		{[]byte{0xDD, 0x21, 0x00, 0xE0}, "LD IX,$E000", 4, FlowNext},
		{[]byte{0xDD, 0x7E, 0x05}, "LD A,(IX+$05)", 3, FlowNext},
		{[]byte{0xFD, 0x66, 0xFE}, "LD H,(IY-$02)", 3, FlowNext},
		{[]byte{0xDD, 0x74, 0x01}, "LD (IX+$01),H", 3, FlowNext},
		{[]byte{0xDD, 0x36, 0x03, 0x7F}, "LD (IX+$03),$7F", 4, FlowNext},
		{[]byte{0xDD, 0x7C}, "LD A,IXH", 2, FlowNext},
		{[]byte{0xDD, 0x26, 0x10}, "LD IXH,$10", 3, FlowNext},
		{[]byte{0xDD, 0x34, 0x02}, "INC (IX+$02)", 3, FlowNext},
		{[]byte{0xDD, 0x86, 0x04}, "ADD A,(IX+$04)", 3, FlowNext},
		{[]byte{0xDD, 0x09}, "ADD IX,BC", 2, FlowNext},
		{[]byte{0xDD, 0xE5}, "PUSH IX", 2, FlowNext},
		{[]byte{0xDD, 0xCB, 0x02, 0x46}, "BIT 0,(IX+$02)", 4, FlowNext},
		{[]byte{0xFD, 0xCB, 0xFF, 0xFE}, "SET 7,(IY-$01)", 4, FlowNext},
		{[]byte{0xDD, 0xCB, 0x01, 0x00}, "RLC (IX+$01),B", 4, FlowNext},
		{[]byte{0xDD, 0xDD}, "NOP*", 1, FlowNext},
		{[]byte{0xEB}, "EX DE,HL", 1, FlowNext},
		{[]byte{0xE3}, "EX (SP),HL", 1, FlowNext},
		{[]byte{0xD3, 0x98}, "OUT ($98),A", 2, FlowNext},
	}
	for _, c := range cases {
		in := Decode(fetch(c.b, 0x4000), 0x4000, nil)
		if in.Text != c.text || in.Len != c.n || in.Flow != c.flow {
			t.Errorf("% X: got %q len %d flow %d, want %q len %d flow %d",
				c.b, in.Text, in.Len, in.Flow, c.text, c.n, c.flow)
		}
	}
}

// Every byte sequence must decode to 1-4 bytes without panicking.
func TestDecodeAllLengths(t *testing.T) {
	for _, pre := range [][]byte{nil, {0xCB}, {0xED}, {0xDD}, {0xFD}, {0xDD, 0xCB, 0x00}, {0xFD, 0xCB, 0x00}} {
		for op := 0; op < 256; op++ {
			b := append(append([]byte{}, pre...), byte(op), 0x00, 0x00)
			in := Decode(fetch(b, 0), 0, nil)
			if in.Len < 1 || in.Len > 4 {
				t.Errorf("% X: length %d", b[:len(pre)+1], in.Len)
			}
		}
	}
}
