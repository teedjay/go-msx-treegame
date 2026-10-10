# go-msx-treegame

A native Go port of a 16KB MSX1 cartridge game (a 1984 tree-climbing
platformer). This is **not an emulator**: no Z80 code runs at runtime. Each
original routine is read, understood and rewritten as ordinary Go against a
small hardware-abstraction layer.

Status: early. Tooling (disassembler, asset extraction) is in progress.

## Requirements

- Go 1.27+
- Your own copy of the original ROM, placed in the repository root as:

  `Magical Tree - Konami (1984) [RC-713] [1393].rom`

  (16384 bytes, SHA-1 `f4b58fecbcbc8cd307c34806ecece2a204c26c65`)

## Tools

| Command | Purpose |
| --- | --- |
| `go run ./cmd/disasm` | Generate `docs/listing.asm` (gitignored) from the ROM |

## Legal

This is an unofficial, non-commercial fan project. It is not affiliated with
or endorsed by the original rights holder. The repository contains only
original source code; it does not include or distribute the ROM, any data
extracted from it, or a disassembly. The engine executes no original code.
Rights holders with concerns are invited to get in touch. See [NOTICE](NOTICE).
