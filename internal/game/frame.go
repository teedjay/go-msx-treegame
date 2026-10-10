package game

// Boot runs the cartridge's INIT routine.
//
// Ported from $408E-$40BD (High). The original also installs JP $403B in
// H.TIMI and sets SP=$E400; in the port the caller drives Frame instead.
// RAM[$005] is the busy flag: set while the VDP is initialised with
// interrupts off, so no tick can run.
func (c *Console) Boot() {
	c.Input.ReleaseAll()
	c.RAM = [0x400]byte{}
	c.RAM[0x005] = 1
	c.initVDP()
	c.RAM[0x005] = 0
}

// initVDP silences the PSG mixer, starts sound $9F, clears VRAM and writes
// the VDP registers.
//
// Ported from $4507-$4518 (High), with $40BE-$40C5 inlined: $40BE's
// RES 6,(HL) targets the ROM register table and has no effect, then it
// jumps to $7A4E with A=$9F.
func (c *Console) initVDP() {
	c.setMixer(0xB8)
	c.playSound(0x9F)
	c.fillVRAM(0x0000, 0x4000, 0)
	c.writeVDPRegs()
}

// Frame runs one vertical-blank interrupt: the sound sequencer always,
// then a game tick unless the busy flag is set.
//
// Ported from $403B-$4057 (High). In the original a tick that takes longer
// than a frame leaves the busy flag set across the next interrupt(s), so
// those run sound only; the port runs every tick within one Frame. See
// docs/ANALYSIS.md, "Tick pacing".
func (c *Console) Frame() {
	c.SoundTick()
	if c.RAM[0x005]&1 != 0 {
		return
	}
	c.BeginTick()
	c.RunTick()
}

// SoundTick runs the sequencer once ($403F: CALL $7AFB).
func (c *Console) SoundTick() { c.soundTick() }

// BeginTick sets the busy flag ($4049: INC (HL)). Separate from RunTick so
// cmd/refcompare can compare state at $404B, where the reference is taken.
func (c *Console) BeginTick() { c.RAM[0x005]++ }

// RunTick runs the game tick and clears the busy flag ($404B-$4056).
func (c *Console) RunTick() {
	c.readPlayerInput()
	c.dispatchState()
	c.RAM[0x005] = 0
}

// dispatchState runs the handler for the current state RAM[$000], then the
// continuation the original pushes before dispatching: every handler's RET
// lands in it. In state 8 nothing is pushed and the RET returns to the
// interrupt handler.
//
// Ported from $40C6-$40E1 (High) with the table at $40E2 (16 entries).
// The continuation is chosen, and the state-8 test made, before the
// handler runs.
func (c *Console) dispatchState() {
	c.RAM[0x003]++
	cont := c.attractInput // $458A
	if c.RAM[0x002]&0x40 != 0 {
		cont = c.blinkPlayerText // $43D7
	}
	st := c.RAM[0x000]
	stateHandlers[st&0x0F](c)
	if st != 8 {
		cont()
	}
}

// stateHandlers is the table at $40E2, indexed by RAM[$000].
var stateHandlers = [16]func(*Console){
	(*Console).state0,  // $4102 load title graphics
	(*Console).state1,  // $410F scroll the logo up
	(*Console).state2,  // $4121 hold, then clear
	(*Console).state3,  // $4132 advance
	(*Console).state4,  // $4134 reveal the title, draw the menu
	(*Console).state5,  // $413C menu: blink the pointer
	(*Console).state6,  // $4145 wipe, then start the demo
	(*Console).state7,  // $414E demo
	(*Console).state8,  // $422D game start: title with the chosen line blinking
	(*Console).state9,  // $415A lose a life / set up the stage
	(*Console).state10, // $417D wait for the tune, start
	(*Console).state11, // $41A3 play
	(*Console).state12, // $41B4 out of lives?
	(*Console).state13, // $41EE swap players
	(*Console).state14, // $4202 game over wait
	(*Console).state15, // $4222 wait, then state 9
}

// todoState stands in for a state handler that is not yet ported.
func todoState(addr uint16) func(*Console) {
	return func(c *Console) {
		// TODO: not yet ported, see the handler at addr.
		c.Unported = addr
	}
}

// advanceState sets the state timer RAM[$004] and moves to the next state.
//
// Ported from $4199-$41A2 (High): $4199 uses A=$20, $419B takes A, $419E
// only increments RAM[$000].
func (c *Console) advanceState(timer byte) {
	c.RAM[0x004] = timer
	c.nextState()
}

// nextState increments RAM[$000]. Ported from $419E-$41A2 (High).
func (c *Console) nextState() { c.RAM[0x000]++ }
