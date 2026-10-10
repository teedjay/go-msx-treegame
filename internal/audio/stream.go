// Package audio turns PSG register snapshots into a 16-bit stereo PCM
// stream for playback or WAV files.
package audio

import (
	"encoding/binary"
	"math"

	"go-msx-treegame/internal/psg"
)

// Stream is an endless io.Reader of 16-bit little-endian stereo samples
// synthesized from the PSG's current registers.
type Stream struct {
	PSG  *psg.PSG
	buf  []float64
	prev float64 // DC-blocking filter state
	out  float64
}

// NewStream returns a stream at rate Hz.
func NewStream(rate int) *Stream { return &Stream{PSG: psg.New(rate)} }

// Read implements io.Reader.
func (s *Stream) Read(p []byte) (int, error) {
	n := len(p) / 4
	if cap(s.buf) < n {
		s.buf = make([]float64, n)
	}
	s.buf = s.buf[:n]
	s.PSG.Generate(s.buf)
	for i, v := range s.buf {
		// One-pole DC blocker: the PSG output is unipolar.
		s.out = v - s.prev + 0.995*s.out
		s.prev = v
		x := int16(math.Max(-1, math.Min(1, s.out*0.6)) * 32767)
		binary.LittleEndian.PutUint16(p[i*4:], uint16(x))
		binary.LittleEndian.PutUint16(p[i*4+2:], uint16(x))
	}
	return n * 4, nil
}
