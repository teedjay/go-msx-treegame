package main

// Hints feed confirmed knowledge back into the recursive descent. They hold
// addresses and counts only, never ROM bytes. Add to them as code is
// confirmed and re-run the disassembler.

// entries are known code entry points.
var entries = []uint16{
	0x408E, // cartridge INIT (header word at $4002)
	0x403B, // H.TIMI VBlank handler, installed by INIT at $4090-$409A
	// Continuations PUSHed by the state dispatcher at $40C6 before
	// CALL JumpTableA, so every state handler's RET lands in one of them.
	// $43D7 when RAM[$002] bit 6 is set, else $458A; skipped in state 8.
	0x43D7,
	0x458A,
	// Callbacks passed in DE to the loop at $6AFD, which calls them via
	// $6B29 (EX DE,HL / JP (HL)).
	0x6B2B, // DE set at $6AFA
	0x6DAB, // DE set at $6DA3
}

// inlineTableCallees are routines that pop their return address and use it
// as a jump table base: the bytes after each CALL are DW entries, not code.
var inlineTableCallees = map[uint16]bool{
	0x4061: true, // ADD A,A / POP HL / CALL $4031 / LD E,(HL) ... JP (HL)
}

// table is a confirmed jump table of count code addresses.
type table struct {
	addr  uint16
	count int
}

// tables lists confirmed jump tables (inline or standalone). An inline table
// listed here overrides the growth heuristic.
var tables = []table{
	// Top-level state machine, indexed by RAM[$000]. High: 16 entries end
	// exactly at the first target $4102 (tight packing).
	{0x40E2, 16},
	// Dispatch at $61A1, indexed by RAM[$1B2]. High for the length: 19
	// entries end exactly at $61D1, the continuation PUSHed at $61A4.
	// The index range of RAM[$1B2] is not yet confirmed.
	{0x61AB, 19},
}

// names are labels for confirmed routines and data. Inside the ROM they
// replace the generic Lxxxx label; outside it they name BIOS calls and
// system variables.
var names = map[uint16]string{
	// Confirmed routines in this ROM.
	0x4031: "AddAtoHL",   // ADD A,L / LD L,A / RET NC / INC H / RET
	0x4036: "AddAtoDE",   // ADD A,E / LD E,A / RET NC / INC D / RET
	0x4061: "JumpTableA", // jump via inline DW table indexed by A
	0x403B: "VBlank",
	0x408E: "Init",
	0x6B29: "CallDE", // EX DE,HL / JP (HL)

	// MSX BIOS.
	0x0000: "CHKRAM",
	0x0006: "VDP.DR",
	0x0007: "VDP.DW",
	0x0038: "KEYINT",
	0x0041: "DISSCR",
	0x0044: "ENASCR",
	0x0047: "WRTVDP",
	0x004A: "RDVRM",
	0x004D: "WRTVRM",
	0x0056: "FILVRM",
	0x0059: "LDIRMV",
	0x005C: "LDIRVM",
	0x0090: "GICINI",
	0x0093: "WRTPSG",
	0x0096: "RDPSG",
	0x013E: "RDVDP",
	0x0141: "SNSMAT",
	0x0156: "KILBUF",

	// MSX system variables and hooks.
	0xF3E0: "RG1SAV",
	0xF3E7: "STATFL",
	0xFD9A: "H.TIMI",
	0xFD9F: "H.KEYI",
}
