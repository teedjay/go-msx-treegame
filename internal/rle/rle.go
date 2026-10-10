// Package rle decodes the cartridge's run-length streams, which the original
// writes straight into VRAM.
//
// Ported from the VRAM stream writer at $44C8-$44E9 (High: traced
// instruction by instruction). The colour-substituting variant at
// $615D-$619F uses the same format (High).
//
// A stream is a sequence of control bytes n:
//
//	$00        end of stream
//	$80        a new 2-byte little-endian VRAM address follows (gap marker)
//	$01-$7F    repeat the next byte n times
//	$81-$FF    copy the next n&$7F bytes literally
//
// The repeat/literal split comes from the CP C at $44DF: equal means bit 7
// was clear and selects the fill at $446A (one source byte), unequal selects
// the copy at $4472 (n source bytes).
package rle

import "fmt"

// Segment is a contiguous run of bytes written at one VRAM address.
type Segment struct {
	VRAM uint16
	Data []byte
}

// Stream is a decoded stream.
type Stream struct {
	Start    uint16 // ROM address of the first byte
	End      uint16 // ROM address one past the terminating $00
	Segments []Segment
}

// Fetcher returns the byte at a CPU address.
type Fetcher func(addr uint16) byte

// Decode decodes the stream at addr. If withAddr is true the stream starts
// with a VRAM address (the $44C8 entry); otherwise writing starts at dest
// (the $44CC entry, DE set by the caller).
func Decode(f Fetcher, addr uint16, withAddr bool, dest uint16) (*Stream, error) {
	s := &Stream{Start: addr}
	p := addr
	word := func() uint16 {
		v := uint16(f(p)) | uint16(f(p+1))<<8
		p += 2
		return v
	}
	if withAddr {
		dest = word()
	}
	cur := Segment{VRAM: dest}
	for n := 0; ; n++ {
		if n > 0x10000 {
			return nil, fmt.Errorf("rle: stream at $%04X does not terminate", addr)
		}
		c := f(p)
		p++
		count := int(c & 0x7F)
		switch {
		case c == 0x00:
			s.Segments = appendSeg(s.Segments, cur)
			s.End = p
			return s, nil
		case c == 0x80:
			s.Segments = appendSeg(s.Segments, cur)
			cur = Segment{VRAM: word()}
		case c&0x80 == 0:
			v := f(p)
			p++
			for i := 0; i < count; i++ {
				cur.Data = append(cur.Data, v)
			}
		default:
			for i := 0; i < count; i++ {
				cur.Data = append(cur.Data, f(p))
				p++
			}
		}
	}
}

func appendSeg(ss []Segment, s Segment) []Segment {
	if len(s.Data) == 0 {
		return ss
	}
	return append(ss, s)
}

// Size returns the total number of bytes the stream writes.
func (s *Stream) Size() int {
	n := 0
	for _, g := range s.Segments {
		n += len(g.Data)
	}
	return n
}
