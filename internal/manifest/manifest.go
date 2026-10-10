// Package manifest lists every asset the port takes from the cartridge:
// where it lives, how it is stored, and how to rebuild it. cmd/extract uses
// it to write assets/*.bin from a user-supplied ROM; the engine uses it to
// know where each asset goes in VRAM. It holds addresses and lengths only,
// never ROM bytes.
package manifest

import (
	"fmt"

	"go-msx-treegame/internal/rle"
	"go-msx-treegame/internal/rom"
)

// Kind is how an asset is stored in the ROM.
type Kind int

const (
	// Raw is a plain byte range: Addr, Len.
	Raw Kind = iota
	// RLE is a stream at Addr whose first word is its VRAM address. Each
	// decoded segment must match Segments exactly.
	RLE
	// RLEAt is a stream at Addr without a leading address; the caller
	// supplies the destination. It must decode to one segment of Len bytes.
	RLEAt
	// Derived is computed from another asset (Source) by Derive.
	Derived
)

// Segment is one expected VRAM write of an RLE asset.
type Segment struct {
	VRAM uint16
	Len  int
}

// Entry is one asset. An RLE entry with several segments produces one file
// per segment, named Name, Name_1, Name_2, ...
type Entry struct {
	Name     string
	Kind     Kind
	Addr     uint16
	Len      int       // Raw, RLEAt
	Segments []Segment // RLE
	// End is the ROM address one past the stream; checked to catch a
	// decoder that runs short or long (tight packing). RLE and RLEAt.
	End    uint16
	Source string                 // Derived
	Derive func(src []byte) []byte // Derived
	// Confidence: High, Medium or Low, with the evidence.
	Confidence string
}

// Files returns the output file names of an entry, in segment order.
func (e *Entry) Files() []string {
	if e.Kind != RLE || len(e.Segments) <= 1 {
		return []string{e.Name}
	}
	fs := []string{e.Name}
	for i := 1; i < len(e.Segments); i++ {
		fs = append(fs, fmt.Sprintf("%s_%d", e.Name, i))
	}
	return fs
}

// Extract rebuilds every asset. It returns file name -> contents, or an
// error naming the first entry whose decode does not match the manifest.
func Extract(r *rom.ROM) (map[string][]byte, error) {
	out := map[string][]byte{}
	for i := range Entries {
		e := &Entries[i]
		switch e.Kind {
		case Raw:
			b := make([]byte, e.Len)
			for j := range b {
				b[j] = r.At(e.Addr + uint16(j))
			}
			out[e.Name] = b
		case RLE, RLEAt:
			s, err := rle.Decode(r.At, e.Addr, e.Kind == RLE, 0)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", e.Name, err)
			}
			if s.End != e.End {
				return nil, fmt.Errorf("%s: stream ends at $%04X, manifest says $%04X", e.Name, s.End, e.End)
			}
			if e.Kind == RLEAt {
				if len(s.Segments) != 1 || len(s.Segments[0].Data) != e.Len {
					return nil, fmt.Errorf("%s: decoded %d segment(s), %d bytes; want 1 segment of %d", e.Name, len(s.Segments), s.Size(), e.Len)
				}
				out[e.Name] = s.Segments[0].Data
				continue
			}
			if len(s.Segments) != len(e.Segments) {
				return nil, fmt.Errorf("%s: decoded %d segments, manifest lists %d", e.Name, len(s.Segments), len(e.Segments))
			}
			for j, g := range s.Segments {
				want := e.Segments[j]
				if g.VRAM != want.VRAM || len(g.Data) != want.Len {
					return nil, fmt.Errorf("%s segment %d: decoded VRAM $%04X len %d, manifest says $%04X len %d",
						e.Name, j, g.VRAM, len(g.Data), want.VRAM, want.Len)
				}
				out[e.Files()[j]] = g.Data
			}
		case Derived:
			src, ok := out[e.Source]
			if !ok {
				return nil, fmt.Errorf("%s: source %q not extracted yet (order matters)", e.Name, e.Source)
			}
			out[e.Name] = e.Derive(src)
		}
	}
	return out, nil
}

// Lookup returns the entry with the given name.
func Lookup(name string) *Entry {
	for i := range Entries {
		if Entries[i].Name == name {
			return &Entries[i]
		}
	}
	return nil
}
