package game

// Entry points for the preview tools. They drive the ported routines; they
// are not part of the game's own flow.

// DebugLoadGraphics sets the VDP registers and loads every tile set and
// sprite set the way the original does, with RAM[$05D] = stageColour.
func (c *Console) DebugLoadGraphics(stageColour byte) {
	c.writeVDPRegs()
	c.RAM[0x05D] = stageColour
	c.loadFontWhite()
	c.loadStageGraphics()
}

// DebugPlaySound starts sound id through the sequencer ($7A4E).
func (c *Console) DebugPlaySound(id byte) { c.playSound(id) }

// DebugSoundBusy reports whether any sound channel is active.
func (c *Console) DebugSoundBusy() bool {
	return c.RAM[0x012] != 0 || c.RAM[0x01D] != 0 || c.RAM[0x028] != 0
}
