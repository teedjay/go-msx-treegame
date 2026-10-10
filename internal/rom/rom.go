// Package rom loads the user-supplied cartridge image and verifies that it
// is the exact dump every address in this project refers to.
package rom

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
)

const (
	// DefaultPath is the filename users are told to place in the repo root.
	DefaultPath = "Magical Tree - Konami (1984) [RC-713] [1393].rom"
	// Base is the CPU address the cartridge is mapped at.
	Base = 0x4000
	// Size is the cartridge size in bytes.
	Size = 0x4000
	// SHA1 of the expected dump.
	SHA1 = "f4b58fecbcbc8cd307c34806ecece2a204c26c65"
)

// ROM is a loaded cartridge image.
type ROM struct {
	Data []byte
	// Verified is true when the image matches SHA1.
	Verified bool
}

// Load reads the image at path. A checksum mismatch is not an error, but
// the caller should warn: every address in the notes assumes this dump.
func Load(path string) (*ROM, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) != Size {
		return nil, fmt.Errorf("%s: size %d, want %d", path, len(b), Size)
	}
	sum := sha1.Sum(b)
	return &ROM{Data: b, Verified: hex.EncodeToString(sum[:]) == SHA1}, nil
}

// Contains reports whether a CPU address lies inside the cartridge.
func Contains(addr uint16) bool { return addr >= Base && int(addr) < Base+Size }

// At returns the byte at a CPU address, or 0xFF outside the cartridge
// (an unmapped MSX slot reads as $FF).
func (r *ROM) At(addr uint16) byte {
	if !Contains(addr) {
		return 0xFF
	}
	return r.Data[addr-Base]
}

// Word returns the little-endian 16-bit value at a CPU address.
func (r *ROM) Word(addr uint16) uint16 {
	return uint16(r.At(addr)) | uint16(r.At(addr+1))<<8
}

// WarnIfUnverified prints a warning to stderr for an unexpected dump.
func (r *ROM) WarnIfUnverified() {
	if !r.Verified {
		fmt.Fprintf(os.Stderr, "warning: ROM SHA-1 does not match %s; addresses in the notes may not apply\n", SHA1)
	}
}
