// Package psg synthesizes AY-3-8910 output from register values: three
// square-wave tone channels, a noise generator, the mixer, the
// logarithmic volume table and the envelope generator.
//
// It is a sound chip model fed by the ported sound sequencer's register
// writes; it runs no original code.
package psg

import "sync"

// Clock is the MSX PSG clock in Hz.
const Clock = 1789772.5

// volumeTable is the AY-3-8910's 16-step amplitude curve, normalised.
var volumeTable = [16]float64{
	0, 0.0137, 0.0205, 0.0291, 0.0423, 0.0618, 0.0847, 0.1369,
	0.1691, 0.2647, 0.3527, 0.4499, 0.5704, 0.6873, 0.8482, 1.0,
}

// PSG is the chip state. SetRegs may be called from another goroutine
// than Generate.
type PSG struct {
	mu   sync.Mutex
	regs [16]byte

	rate      float64 // output sample rate
	toneCnt   [3]float64
	toneOut   [3]bool
	noiseCnt  float64
	lfsr      uint32
	noiseOut  bool
	envCnt    float64
	envStep   int
	envHold   bool
	envShape  byte
	envReload bool
}

// New returns a PSG producing samples at rate Hz.
func New(rate int) *PSG {
	return &PSG{rate: float64(rate), lfsr: 1}
}

// SetRegs replaces the register state. A change of register 13 restarts
// the envelope, as a write to it does on the chip.
func (p *PSG) SetRegs(r [16]byte) {
	p.mu.Lock()
	if r[13] != p.regs[13] {
		p.envReload = true
	}
	p.regs = r
	p.mu.Unlock()
}

// Generate fills buf with mono samples in -1..1.
func (p *PSG) Generate(buf []float64) {
	p.mu.Lock()
	r := p.regs
	if p.envReload {
		p.envReload = false
		p.envShape = r[13] & 0x0F
		p.envStep = 0
		p.envHold = false
		p.envCnt = 0
	}
	p.mu.Unlock()

	step := Clock / 16 / p.rate // chip ticks per sample (tone/noise units)
	var period [3]float64
	for ch := 0; ch < 3; ch++ {
		pr := int(r[ch*2]) | int(r[ch*2+1]&0x0F)<<8
		if pr == 0 {
			pr = 1
		}
		period[ch] = float64(pr)
	}
	np := float64(r[6] & 0x1F)
	if np == 0 {
		np = 1
	}
	ep := float64(int(r[11]) | int(r[12])<<8)
	if ep == 0 {
		ep = 1
	}
	mixer := r[7]

	for i := range buf {
		for ch := 0; ch < 3; ch++ {
			p.toneCnt[ch] += step
			for p.toneCnt[ch] >= period[ch] {
				p.toneCnt[ch] -= period[ch]
				p.toneOut[ch] = !p.toneOut[ch]
			}
		}
		p.noiseCnt += step
		for p.noiseCnt >= np*2 {
			p.noiseCnt -= np * 2
			bit := (p.lfsr ^ p.lfsr>>3) & 1
			p.lfsr = p.lfsr>>1 | bit<<16
			p.noiseOut = p.lfsr&1 != 0
		}
		p.envCnt += step / 16 // envelope steps at clock/256
		for p.envCnt >= ep {
			p.envCnt -= ep
			p.advanceEnvelope()
		}
		env := p.envelopeLevel()

		var s float64
		for ch := 0; ch < 3; ch++ {
			tone := p.toneOut[ch] || mixer&(1<<ch) != 0
			noise := p.noiseOut || mixer&(8<<ch) != 0
			if !(tone && noise) {
				continue
			}
			v := r[8+ch]
			level := int(v & 0x0F)
			if v&0x10 != 0 {
				level = env
			}
			s += volumeTable[level]
		}
		buf[i] = s/3*2 - 0 // 0..2/3 range; centred by the caller's DC filter
	}
}

func (p *PSG) advanceEnvelope() {
	if p.envHold {
		return
	}
	p.envStep++
	if p.envStep < 16 {
		return
	}
	s := p.envShape
	cont, alt, hold := s&8 != 0, s&2 != 0, s&1 != 0
	switch {
	case !cont:
		p.envHold = true
		p.envStep = 16 // level 0 after one ramp
	case hold:
		p.envHold = true
		if alt {
			p.envStep = 32 // hold at the opposite end
		} else {
			p.envStep = 15 + 16 // hold at the end of the ramp
		}
	default:
		p.envStep = 0
		if alt {
			p.envShape ^= 4 // flip direction each cycle
		}
	}
}

func (p *PSG) envelopeLevel() int {
	attack := p.envShape&4 != 0
	switch {
	case p.envStep < 16:
		if attack {
			return p.envStep
		}
		return 15 - p.envStep
	case p.envStep == 16:
		return 0
	case p.envStep == 32:
		if attack {
			return 0
		}
		return 15
	default:
		if attack {
			return 15
		}
		return 0
	}
}
