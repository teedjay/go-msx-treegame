package game

// Sound sequencer. It runs once per interrupt (even when the game tick is
// skipped because the previous one overran) and writes PSG registers into
// c.PSG. The game logic reads its channel state (e.g. RAM[$012] to wait for
// a tune to finish), so it is ported faithfully, not approximated.
//
// Channel layout, three channels of 11 bytes at $E010, $E01B, $E026
// (High for the offsets, Medium for the names):
//
//	+0 duration counter     +1 note length      +2 sound id (0 = idle;
//	bit 7 = tune format, bit 6 = keep noise)    +3/+4 data pointer
//	+5 octave shift         +6 volume           +7 current volume
//	+8 decay counter        +9 repeat counter   +A tempo multiplier
//
// The PSG tone register pair for a channel is selected by c (1, 3, 5: the
// coarse register; fine is c-1); its volume register is RRCA(c)+$88.

const (
	sndChan0  = 0x010 // RAM offset of channel 0
	sndChanSz = 11
	sndMixer  = 0x031 // RAM[$031]: shadow of PSG register 7
)

// writePSG is BIOS WRTPSG ($0093).
func (c *Console) writePSG(reg, v byte) {
	c.PSG[reg&0x0F] = v
}

// PlaySound starts sound id a, subject to channel priority.
//
// Ported from $7A4E-$7A55 (High).
func (c *Console) playSound(a byte) {
	c.startSound(a, 0)
}

// startSound assigns sound id a to its channels. With d=0 the priority
// test applies; with d=1 (restart from a $FE repeat) it does not.
//
// Ported from $7A56-$7AB7 (High). $7A77-$7A7B is unreachable (no branch
// targets it).
func (c *Console) startSound(a, d byte) {
	id := a
	n := 2
	ch := uint16(0x012) // channel 0, +2
	switch {
	case a < 0x8C:
		n = 1
		a &= 0x3F
		switch a {
		case 0x09:
		case 0x0A:
			ch = 0x01D // channel 1, +2
		default:
			ch = 0x028 // channel 2, +2
		}
	case a < 0x90:
	default:
		n = 3
	}
	d--
	if d != 0 {
		// Priority: only a higher id (low 6 bits) replaces the current one.
		cur := c.RAM[ch] & 0x3F
		a = id & 0x3F
		if a == cur || a < cur {
			return
		}
		if a >= 0x19 {
			id = a // ids from $19 are stored without their high bits
		}
	}
	de := 0x7C93 + uint16(a+a) // ADD A,A: 8-bit
	hl := ch - 2
	for ; n > 0; n-- {
		c.RAM[hl] = 1
		c.RAM[hl+1] = 1
		c.RAM[hl+2] = id
		c.RAM[hl+3] = romByte(de)
		c.RAM[hl+4] = romByte(de + 1)
		// LD A,5 / ADD A,L / LD L,A: low byte only.
		hl = hl&0xFF00 | uint16(byte(hl+4)+5)
		c.RAM[hl] = 0 // +9 repeat counter
		hl += 2
		de += 2
	}
}

// setNoise turns noise for PSG channel pc off (d=1) or on (d=0), and keeps
// tone B on exactly when noise B is off.
//
// Ported from $7AD5-$7AF9 (High).
func (c *Console) setNoise(pc, d byte) {
	a := pc
	if a != 1 {
		a--
	}
	a = a<<3 | a>>5 // RLCA x3 on values <= 4
	e := c.RAM[sndMixer]
	if d == 1 {
		a |= e
	} else {
		a = ^a & e
	}
	a |= 0x02
	if a&0x10 != 0 {
		a &^= 0x02
	}
	c.setMixer(a)
}

// setMixer stores and writes PSG register 7. Ported from $7AF2-$7AFA (High).
func (c *Console) setMixer(a byte) {
	c.RAM[sndMixer] = a
	c.writePSG(7, a)
}

// soundTick advances all three channels by one interrupt.
//
// Ported from $7AFB-$7B24 (High).
func (c *Console) soundTick() {
	c.setMixer(c.RAM[sndMixer])
	pc := byte(1)
	for ix := uint16(sndChan0); ix < sndChan0+3*sndChanSz; ix += sndChanSz {
		id := c.RAM[ix+2]
		if id == 0x0B {
			c.sweepSound(ix)
		}
		if id != 0 {
			c.updateChannel(ix, pc, id)
		}
		pc += 2
	}
}

// sweepSound points channel ix at the RAM sequence $E032-$E035, copying it
// from ROM the first time and raising its period by 2 on later calls.
//
// Ported from $7B25-$7B4E (High).
func (c *Console) sweepSound(ix uint16) {
	var ptr uint16
	if c.RAM[0x032] == 0 {
		// LDDR $7B52..$7B4F -> $E035..$E032
		for i := uint16(0); i < 4; i++ {
			c.RAM[0x035-i] = romByte(0x7B52 - i)
		}
		ptr = 0xE032
	} else {
		v := uint16(c.RAM[0x035]) + 2
		c.RAM[0x035] = byte(v)
		if v > 0xFF {
			c.RAM[0x034]++
		}
		ptr = 0xE033
	}
	c.RAM[ix+3] = byte(ptr)
	c.RAM[ix+4] = byte(ptr >> 8)
}

// updateChannel runs one channel for one interrupt.
//
// Ported from $7B53-$7B64 (High).
func (c *Console) updateChannel(ix uint16, pc, id byte) {
	if id&0x40 == 0 {
		c.setNoise(pc, 1)
	}
	if c.RAM[ix+2]&0x80 != 0 {
		c.updateTune(ix, pc)
		return
	}
	c.RAM[ix]--
	if c.RAM[ix] != 0 {
		return
	}
	c.nextEvent(ix, pc)
}

func (c *Console) chanPtr(ix uint16) uint16 {
	return uint16(c.RAM[ix+3]) | uint16(c.RAM[ix+4])<<8
}

func (c *Console) setChanPtr(ix, p uint16) {
	c.RAM[ix+3] = byte(p)
	c.RAM[ix+4] = byte(p >> 8)
}

// nextEvent reads the channel's next sequence event.
//
// Ported from $7B65-$7BCC (High).
func (c *Console) nextEvent(ix uint16, pc byte) {
	hl := c.chanPtr(ix)
	a := c.mem(hl)
	switch {
	case a == 0xFE:
		c.repeatSound(ix, hl, pc)
		return
	case a == 0xFF:
		c.endSound(ix, pc)
		return
	}
	if c.RAM[ix+2]&0x80 != 0 {
		c.tuneNote(ix, hl, pc)
		return
	}
	if a&0xF0 == 0x20 { // $2n: note length n
		c.RAM[ix+1] = a & 0x0F
		hl++
	}
	a = c.mem(hl)
	if a&0xF0 == 0x10 { // $1n: noise period, noise on
		c.writePSG(6, a&0x1F)
		c.setNoise(pc, 0)
		hl++
		a = c.mem(hl)
	}
	if c.RAM[ix+2]&0x40 != 0 && pc == 3 {
		// Channel B of a bit-6 sound: a bare volume byte.
		hl++
		c.setChanPtr(ix, hl)
		c.noteOn(ix, pc, a)
		return
	}
	// $vp pp: volume v, 12-bit period p.
	vol := a & 0xF0
	period := uint16(a^vol)<<8 | uint16(c.mem(hl+1))
	hl += 2
	c.setChanPtr(ix, hl)
	c.writeTone(pc, period)
	c.noteOn(ix, pc, vol>>4|vol<<4) // RRCA x4
}

// noteOn reloads the duration and decay counters and sets the volume.
//
// Ported from $7BCD-$7BDA + $7C0D-$7C14 (High).
func (c *Console) noteOn(ix uint16, pc, vol byte) {
	n := c.RAM[ix+1]
	c.RAM[ix] = n
	c.RAM[ix+8] = n + 3
	c.writeVolume(pc, vol)
}

// writeVolume writes the channel's volume register.
// Ported from $7C0D-$7C14 (High).
func (c *Console) writeVolume(pc, vol byte) {
	c.writePSG((pc>>1|pc<<7)+0x88, vol) // RRCA, ADD A,$88 (8-bit)
}

// endSound silences and frees the channel.
//
// Ported from $7BDB-$7BEA (High).
func (c *Console) endSound(ix uint16, pc byte) {
	c.RAM[ix+9] = 0
	c.setNoise(pc, 1)
	c.RAM[ix+2] = 0
	c.writeVolume(pc, 0)
}

// repeatSound handles $FE n: restart the sound until it has played n times.
//
// Ported from $7AB8-$7AD4 (High). The restart goes through startSound with
// d=1, which also clears the repeat counter of every channel of the sound;
// this channel's counter is then set to the new count.
func (c *Console) repeatSound(ix, hl uint16, pc byte) {
	n := c.mem(hl + 1)
	a := c.RAM[ix+9] + 1
	if a == n {
		c.endSound(ix, pc)
		return
	}
	if (a-n)&0x80 == 0 { // JP M not taken
		a--
	}
	c.startSound(c.RAM[ix+2], 1)
	c.RAM[ix+9] = a
}

// updateTune runs a tune-format channel: hold the note while decaying the
// volume, then read the next note.
//
// Ported from $7BEB-$7C0C (High).
func (c *Console) updateTune(ix uint16, pc byte) {
	c.RAM[ix]--
	if c.RAM[ix] == 0 {
		c.nextEvent(ix, pc)
		return
	}
	c.RAM[ix+8]--
	a := c.RAM[ix+8]
	if a != c.RAM[ix] {
		c.RAM[ix+8]--
	} else if a >= 3 {
		return
	}
	v := c.RAM[ix+7] - 1
	if v&0x80 != 0 {
		return
	}
	c.RAM[ix+7] = v
	c.writeVolume(pc, v)
}

// tuneNote reads one tune-format note: optional $Dn tempo, $Fn volume, $En
// octave, then $nl (note n, length multiplier l).
//
// Ported from $7C16-$7C88 (High).
func (c *Console) tuneNote(ix, hl uint16, pc byte) {
	a := c.mem(hl)
	if a&0xF0 == 0xD0 {
		c.RAM[ix+0x0A] = a & 0x0F
		hl++
		a = c.mem(hl)
	}
	if a >= 0xF0 {
		c.RAM[ix+6] = a & 0x0F
		hl++
		a = c.mem(hl)
	}
	if a >= 0xE0 {
		c.RAM[ix+5] = a & 0x0F
		hl++
		a = c.mem(hl)
	}
	// Length = tempo * (l+1), 8-bit.
	l := a & 0x0F
	length := c.RAM[ix+0x0A]
	for ; l > 0; l-- {
		length += c.RAM[ix+0x0A]
	}
	c.RAM[ix+1] = length
	a = c.mem(hl)
	hl++
	c.setChanPtr(ix, hl)
	note := a >> 4
	vol := note - 0x0C // note 12 is a rest: volume 0
	c.RAM[ix+7] = vol
	if vol != 0 {
		vol = c.RAM[ix+6]
		c.RAM[ix+7] = vol
	}
	c.noteOn(ix, pc, vol)
	period := uint16(romByte(0x7C89 + uint16(note)))
	for s := c.RAM[ix+5]; s > 0; s-- {
		period <<= 1
	}
	c.writeTone(pc, period)
}

// writeTone writes a 12-bit period to the channel's tone registers.
// Ported from $7C7D-$7C88 (High).
func (c *Console) writeTone(pc byte, period uint16) {
	c.writePSG(pc, byte(period>>8))
	c.writePSG(pc-1, byte(period))
}
