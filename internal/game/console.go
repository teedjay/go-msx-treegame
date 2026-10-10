// Package game is the native Go port of the cartridge's engine. No Z80 code
// runs: each routine is rewritten by hand. Every function names the Z80
// address range it was ported from, and states its confidence:
//
//	High   traced instruction by instruction and/or visually confirmed
//	Medium control flow clear, meaning inferred
//	Low    plausible guess
package game

import "go-msx-treegame/internal/msxrender"

// Console is the hardware the original runs on, reduced to what the port
// needs: game RAM, VRAM and the VDP registers.
type Console struct {
	// RAM is $E000-$E3FF. Unconfirmed bytes are addressed as RAM[0x0NN]
	// for $E0NN, the way the original addresses them.
	RAM  [0x400]byte
	VRAM [msxrender.VRAMSize]byte
	Reg  [8]byte
}

// New returns a console in its power-on state.
func New() *Console {
	return &Console{}
}
