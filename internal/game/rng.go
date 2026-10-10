package game

// randR stands in for LD A,R at Z80 address pc. The refresh register
// depends on exact instruction timing, which a native port does not have,
// so the value comes from c.RandR when set (cmd/refcompare replays the
// values recorded from the original) and otherwise from a xorshift
// generator. The original's R counts 0-127 (bit 7 stays 0: the game never
// executes LD R,A), so values are 0-127; callers mask them further.
func (c *Console) randR(pc uint16) byte {
	if c.RandR != nil {
		return c.RandR(pc)
	}
	x := c.rngState
	if x == 0 {
		x = 0x2545F491
	}
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	c.rngState = x
	return byte(x) & 0x7F
}
