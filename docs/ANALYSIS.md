# ROM analysis

The source of truth for what is known about the cartridge. Corrections are
appended, never edited away.

Confidence key: **High** = traced instruction by instruction and/or visually
confirmed. **Medium** = control flow clear, meaning inferred. **Low** =
plausible guess.

## Open issues

1. Confirm the VRAM layout by compositing one pattern+colour pair (needs
   `internal/msxrender`).
2. Find the graphics decompressor, if any. The font at `$45EB` is stored
   **uncompressed**, so the big data blocks may not be compressed either.
   Check each one; do not assume.
3. Trace the per-frame tick (`$403B` → `$4555`, `$40C6`) and name the 16
   top-level states.
4. Confirm the index range of `RAM[$1B2]` (dispatch at `$61A1`).
5. Classify the data blocks listed under "Unclaimed ranges".
6. RNG: `LD A,R` at `$686F $6C42 $704B $7139 $71E6 $7227 $725C $728C
   $72FB`. Document what each draws and the distribution it needs.

## The dump

| | |
| --- | --- |
| Size | 16384 bytes, `$4000-$7FFF`, no mapper |
| CRC32 | `d4d30bcd` |
| MD5 | `ae7c3a41d9b23e7c889fb318203a43d9` |
| SHA-1 | `f4b58fecbcbc8cd307c34806ecece2a204c26c65` |
| SHA-256 | `b757aec09d839b4f8aba078ceda9b9809a8de8dc34f3df375d157fbe0dd8116c` |

## Boot and frame structure

- Header (High): "AB", INIT `$408E`, no BASIC statement/device/text hooks.
- INIT `$408E` (High): `DI`, `IM 1`. It writes `JP $403B` to `H.TIMI`
  (`$FD9A`), sets `SP=$E400`, and zeroes `$E000-$E3FF`. It then sets
  `RAM[$005]=1`, calls `$4507` (VDP init), clears `RAM[$005]`, calls
  `RDVDP`, `EI`, and idles in `JR $` at `$40BC`.
- VBlank `$403B` (Medium): `DI`, `RDVDP` (acknowledges the interrupt),
  `CALL $7AFB`. If `RAM[$005]` bit 0 is set, it returns (busy guard).
  Otherwise it increments `RAM[$005]`, runs with interrupts enabled
  `CALL $4555` and `CALL $40C6`, then clears `RAM[$005]` and returns.
- State dispatcher `$40C6` (High for control flow): it increments
  `RAM[$003]` every frame. It picks a continuation by `RAM[$002]` bit 6
  (`$43D7` if set, else `$458A`) and `PUSH`es it unless `RAM[$000]` = 8.
  It then dispatches on `RAM[$000]` through the 16-entry table at `$40E2`.
  **Every state handler's `RET` falls into the pushed continuation.** In
  state 8 the `RET` goes straight back to `$403B`.

## VRAM layout (High for the derivation, not yet visually confirmed)

Registers R0-R7 are written from the table at `$452C` by the loop at
`$451B`: `02 E2 0E 7F 07 76 03 E4`.

| Table | Address |
| --- | --- |
| Colour table | `$0000` |
| Sprite patterns | `$1800` |
| Pattern generator | `$2000` |
| Name table | `$3800` |
| Sprite attributes | `$3B00` |

The mode is Screen 2 with 16×16 unmagnified sprites and a dark blue
backdrop (colour 4). `$40BE` clears bit 6 of the R1 copy at `$452D`
(display off), so the register table doubles as the shadow copy.

## Code structure notes

| Address | What | Confidence |
| --- | --- | --- |
| `$4031` | `AddAtoHL` | High |
| `$4036` | `AddAtoDE` | High |
| `$4061` | `JumpTableA`: inline DW table after the `CALL`, indexed by A | High |
| `$6B29` | `CallDE`: `EX DE,HL / JP (HL)` | High |
| `$6AFD` | Loop over 39 entries at `RAM[$137]` that invokes the DE callback. Entered with `DE=$6B2B` (from `$6AFA`) or `DE=$6DAB` (from `$6DA3`) | Medium |

## Assets

| Address | What | Compressed? | Confidence |
| --- | --- | --- | --- |
| `$45EB`… | 1-bit 8×8 font, glyphs "0", "1", … and "©" at `$463B` | No | Medium (seen in a linear dump; extent not yet measured) |
| `$426D`… | Text, including "HEIGHT" | No | Low |

## Unclaimed ranges (after the 2026-10-10 disassembler run)

52.4% of the ROM is decoded as code, 70 bytes are jump tables, and 47.1% is
unclaimed. Every large unclaimed range is referenced by `LD HL/DE,nn`
from decoded code, so these ranges are data. Run
`go run ./cmd/disasm -gaps` for the current list with references.
The big blocks are:

- `$45E7-$48CC` (742 bytes): font, then tables referenced from the state
  handlers.
- `$4A7E-$51BB` (1854 bytes)
- `$5412-$60F9` (3304 bytes)
- `$769C-$7804` (361 bytes)
- `$7C89-$7FFF` (887 bytes)

## Corrections

- 2026-10-10: the jump-table trampoline is at `$4061`, not `$405F` as first
  written in CLAUDE.md. `$405F` holds the last two bytes of `JP $0047`.
