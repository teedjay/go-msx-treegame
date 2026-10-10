package game

// Input is the controller hardware as the original sees it. Both fields
// are active-low, like the real ports: a zero bit is a pressed key.
type Input struct {
	// KeyRow is the keyboard matrix read by BIOS SNSMAT. Only rows 7
	// (bit 6 SELECT) and 8 (bit 0 SPACE, 4 LEFT, 5 UP, 6 DOWN, 7 RIGHT) are
	// read by the game.
	KeyRow [11]byte
	// Joy is PSG register 14 for joystick ports 1 and 2: bit 0 up, 1 down,
	// 2 left, 3 right, 4 trigger A, 5 trigger B.
	Joy [2]byte
}

// Controller bits after the original's CPL: the format both readers return.
const (
	BtnUp    = 0x01
	BtnDown  = 0x02
	BtnLeft  = 0x04
	BtnRight = 0x08
	BtnA     = 0x10 // trigger A / SPACE
	BtnB     = 0x20 // trigger B / SELECT
)

// ReleaseAll sets every key and joystick line to released.
func (in *Input) ReleaseAll() {
	for i := range in.KeyRow {
		in.KeyRow[i] = 0xFF
	}
	in.Joy = [2]byte{0xFF, 0xFF}
}

// readJoystick selects joystick port 1 (e=$8F) or 2 (e=$CF) through PSG
// register 15 and returns register 14, inverted and masked to 6 bits.
//
// Ported from $4545-$4554 (High).
func (c *Console) readJoystick(e byte) byte {
	c.writePSG(15, e)
	port := 0
	if c.PSG[15]&0x40 != 0 {
		port = 1
	}
	return ^c.Input.Joy[port] & 0x3F
}

// readKeyboard maps keyboard rows 7 and 8 to the joystick bit layout:
// cursor keys to directions, SPACE to A, SELECT to B.
//
// Ported from $4565-$4589 (High).
func (c *Console) readKeyboard() byte {
	e := rrca(^c.Input.KeyRow[7]) & 0x20 // SELECT (bit 6) -> bit 5
	b := rrca(rrca(^c.Input.KeyRow[8]))
	r := b&0x04 | e // LEFT (bit 4) -> bit 2
	b = rrca(rrca(b))
	r |= b & 0x18 // RIGHT (bit 7) -> bit 3, SPACE (bit 0) -> bit 4
	b = rrca(b)
	r |= b & 0x03 // UP (bit 5) -> bit 0, DOWN (bit 6) -> bit 1
	return r
}

func rrca(a byte) byte { return a>>1 | a<<7 }

// readPlayerInput reads the active player's controller into RAM[$009],
// moving the previous reading to RAM[$008]. RAM[$002] bit 7 selects
// joystick port 2, bit 4 the keyboard.
//
// Ported from $453A-$4544 + $4555-$4564 (High). The joystick is read even
// when the keyboard is selected; that read's PSG register 15 write is kept.
func (c *Console) readPlayerInput() {
	e := byte(0x8F)
	if c.RAM[0x002]&0x80 != 0 {
		e |= 0x40
	}
	a := c.readJoystick(e)
	if c.RAM[0x002]&0x10 != 0 {
		a = c.readKeyboard()
	}
	c.RAM[0x008] = c.RAM[0x009]
	c.RAM[0x009] = a
}
