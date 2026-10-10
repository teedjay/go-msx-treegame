# ROM analysis

The source of truth for what is known about the cartridge. Corrections are
appended, never edited away.

Confidence key: **High** = traced instruction by instruction and/or visually
confirmed. **Medium** = control flow clear, meaning inferred. **Low** =
plausible guess.

## Open issues

1. Coverage: verified so far are game states 0-11 and player states 0,
   2, 3, 4, 5, 6, 8 and 9 (see "Verification"). Not yet exercised against
   the original: losing a life in a game, game over (states 12-15), 2-player
   swaps, the ladder (player state 7), reaching the top of a section and
   stage clear (player states 12-18). Needs real play: record with
   `cmd/play -record` in a KEYBOARD mode and replay into openMSX.
2. `upd6BF0` (`$6C38`): when a special enemy does not move, the original
   tests `BIT 2,E` with E left over from earlier code; the port uses the
   enemy's low y byte. Unverified until a trace includes a stationary
   special enemy.
3. Tick pacing: in the original, a tick that takes longer than a frame
   leaves the busy flag set, and the following interrupts run sound only.
   Tick 0 (title graphics load) spans 76 interrupts, the stage load at the
   end of state 6 spans 106, and some gameplay ticks span 2. The port runs
   one tick per `Frame`, so these pauses are shorter. Decide how to model
   them (a per-tick cost table measured from traces is one option).
3. Confirm the index range of `RAM[$1B2]` (dispatch at `$61A1`).
3. Classify the remaining data blocks: `$4A7E-$51BB` (probably stage or map
   data), `$5412-$5472`, `$769C-$7804`, `$7C89-$7FFF`, and the text strings
   around `$4743-$47EE`.
4. Find what loads sprite patterns `$1A80-$1AFF` (sprites 20-23), if
   anything.
5. RNG: `LD A,R` at `$686F $6C42 $704B $7139 $71E6 $7227 $725C $728C
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

## States (table at `$40E2`, indexed by `RAM[$000]`)

| State | Handler | What | Port | Verified |
| --- | --- | --- | --- | --- |
| 0 | `$4102` | load title graphics (one tick, 76 frames in the original) | `state0` | trace |
| 1 | `$410F` | publisher logo scrolls up a row every other tick, 14 rows | `state1` | trace |
| 2 | `$4121` | hold 256 ticks, clear screen, backdrop black | `state2` | trace |
| 3 | `$4132` | advance | `state3` | trace |
| 4 | `$4134` | reveal title logo one column per tick, draw menu, wait to $34 | `state4` | trace |
| 5 | `$413C` | menu, pointer blinks, 256 ticks | `state5` | trace |
| 6 | `$4145` | column wipe, then stage setup `$53F0` | `state6` | trace up to `$53F0` |
| 7 | `$414E` | demo / play | not yet | |
| 8 | `$422D` | game start (set by the menu) | not yet | |
| 9-15 | `$415A` `$417D` `$41A3` `$41B4` `$41EE` `$4202` `$4222` | not yet traced | | |

Continuations (see "Boot and frame structure"): `$458A` merges both
joysticks and the keyboard in attract mode. A fresh press jumps to the menu
(`$423F`); on the menu a direction moves `RAM[$042]` and a trigger starts a
game with mode `tab_45e7[RAM[$042]]`. `$43D7` blinks the player label every
32 ticks in a game.

## Verification against the original (lockstep trace)

`scripts/refdump.tcl` runs the original in openMSX (C-BIOS, offline oracle)
and records RAM, VRAM and VDP registers at `$404B`, the start of every game
tick, plus the interrupt count. `cmd/refcompare` replays the port in
lockstep, running the sound sequencer once per recorded interrupt, and
reports the first differing byte.

- 2026-10-10: ticks 0-626 of the attract sequence (states 0-6, including
  the title tune's sound channel state) match byte for byte. The port then
  reaches the unported `$53F0`.
- The comparator was checked by flipping one VRAM byte in a copy of the
  trace: it reported exactly that tick and address.
- 2026-10-10: the whole gameplay port matches. Attract trace: 3000 ticks
  including the full scripted demo (player states 0, 2, 3, 5, 6, 8, 9).
  Four scripted keyboard-mode games from the menu (8000 + 3 x 12000
  ticks, pseudo-random input): all match (game states 8-11, player states
  0, 2, 3, 4, 5, 6).
- One divergence was found and fixed on the way: `$E130` kept a byte of
  the mirrored-sprite RAM buffer that `$6115-$6148` builds at `$E0B0`. An
  openMSX write-watchpoint on `$E130` named `$612F`.
- `LD A,R` (9 sites) is replayed from a log the trace script writes; the
  comparator flags any difference in call site or tick.

## Sound (`$7A4E`-`$7C88`, High: traced instruction by instruction)

Ported in `internal/game/sound.go`. Three channels of 11 bytes at `$E010`,
`$E01B`, `$E026`, plus mixer shadow `RAM[$031]`. `$7A4E` starts a sound id
(1-3 channels, priority on the low 6 bits). `$7AFB` runs every interrupt.
There are two sequence formats: effects (`$2n` length, `$1n` noise,
`$vp pp` volume and period, `$FE n` repeat, `$FF` end) and tunes (bit 7
of the id: `$Dn` tempo, `$Fn` volume, `$En` octave, `$nl` note and length,
note 12 = rest). Sound `$0B` runs from a 4-byte sequence copied to RAM
`$E032-$E035` and swept in place by `$7B25`. The game logic reads channel
state (`RAM[$012]`) to wait for tunes to end.

## VRAM layout (High: derived and visually confirmed 2026-10-10)

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
(display off). **But `$452D` is ROM, so the write has no effect**, and the
table is written unchanged by `$451B`. This is a quirk of the original
(probably a leftover from a RAM-resident table during development). The
port reproduces it: the register table is constant.

Visual confirmation: `cmd/gfxpreview` loads the font and stage graphics
through the ported routines and renders all 768 characters. The font reads
correctly ("0123456789©", "A-Z") with the right colours, and the tile set
is coherent. 16×16 sprites decode whole with the TL/BL/TR/BR quadrant order.

## Code structure notes

| Address | What | Confidence |
| --- | --- | --- |
| `$4031` | `AddAtoHL` | High |
| `$4036` | `AddAtoDE` | High |
| `$4061` | `JumpTableA`: inline DW table after the `CALL`, indexed by A | High |
| `$6B29` | `CallDE`: `EX DE,HL / JP (HL)` | High |
| `$6AFD` | Loop over 39 entries at `RAM[$137]` that invokes the DE callback. Entered with `DE=$6B2B` (from `$6AFA`) or `DE=$6DAB` (from `$6DA3`) | Medium |

## VRAM writers (High: traced instruction by instruction)

| Address | Routine | Port |
| --- | --- | --- |
| `$401F` | write A to VRAM at DE | `writeVRAM` |
| `$4028` | read VRAM at DE into A | `readVRAM` |
| `$44EB` / `$44FA` | set write / read address (BIOS `SETWRT`/`SETRD`). The write path reads the port from `$0006` and the read path from `$0007`, which is swapped relative to the BIOS documentation. Harmless, since both are `$98` | n/a |
| `$4459` | fill BC bytes of A at DE | `fillVRAM` |
| `$446E` | copy BC bytes from HL to DE | `copyToVRAM` |
| `$447F` | VRAM→VRAM copy, DE→HL, BC bytes | `copyVRAM` |
| `$448F` | patterns third 0 → 1 → 2 (rolling copy) | `triplicatePatterns` |
| `$44A3` | colours third 0 → 1 → 2 | `triplicateColours` |
| `$44A0` | both of the above | `triplicateAll` |
| `$44B5` | fill $180 colour bytes at DE with A, copy font to DE+$2000 | `loadFontAt` |
| `$44AB` | font in white at chars $30-$5F, then triplicate | `loadFontWhite` |
| `$44C8` | RLE stream to VRAM, starting with its address | `writeRLE` |
| `$44CC` | RLE stream to VRAM at DE | `writeRLEAt` |
| `$60FA` | stage graphics loader | `loadStageGraphics` |
| `$615D` | RLE colour stream with nibble-3 substitution by `RAM[$05D]` | `writeStageColours` |

### RLE format (High)

Shared by `$44C8` and `$615D`. Control byte n: `$00` = end; `$80` = new
2-byte VRAM address follows; `$01-$7F` = repeat the next byte n times;
`$81-$FF` = copy the next n&$7F bytes. The repeat/literal split is decided
by `CP C` at `$44DF` (and `JR Z` around `INC HL` at `$6197` in the colour
variant), not by the ranges alone.

## Assets

The manifest in `internal/manifest/entries.go` is the inventory, with
per-entry confidence and evidence. `go run ./cmd/extract -check`
verifies it. Summary:

| Asset | ROM | Stored | VRAM |
| --- | --- | --- | --- |
| `font_45eb` | `$45EB` +$180 | raw | patterns at colour addr + $2000 |
| `name_47ef` | `$47EF-$47FF` | RLE | names `$394A`, `$396C` |
| `pat_4800` | `$4800-$48C1` | RLE | `$2600` |
| `col_48c2` | `$48C2-$48CC` | RLE, no address | ×16 at `$0600`… (loop at `$4298`) |
| `pat_4923` | `$4923-$49B9` | RLE, no address | `$2300` |
| `pat_5473` | `$5473-$5967` | RLE | `$2038`, `$2300` |
| `col_5968` | `$5968-$5BFD` | RLE (colour-substituted) | `$0008`, `$0300` |
| `spr_5bfe` | `$5BFE` +$140 | raw | `$1800` |
| `spr_5bfe_mirror` | derived | mirror of `spr_5bfe` (`$6115`) | `$1940` |
| `spr_5d3e` | `$5D3E-$60F9` | RLE | `$1B00` |
| `tab_745a` | `$745A` +8 | raw | stage colour table for `RAM[$05D]` |
| `name_75b8` | `$75B8-$75CF` | RLE | names `$394A`, `$398B` |

Notes:

- The font copy is $180 bytes (48 glyphs), but only 43 are real glyphs.
  The last $28 bytes are the text strings at `$4743`. They land in VRAM as
  junk glyphs `$5B-$5F`, which are presumably never displayed.
- `$4743`… text format (`$4079`/`$407B`): a 2-byte VRAM address, then
  characters, then `$FE` + new address, or `$FF` to end. Characters are
  ANDed with C (`$FF` to draw, `$00` to erase via `$4075`).
- State handler `$427D` loads the font twice: white at chars `$30+`, and
  cyan (`$70`) at `$1600`/`$3600` (third 2, chars `$C0+`). It then loads
  `pat_4800` and 16 copies of `col_48c2`.

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

## Extensions (not in the original)

- Level-select cheat (`internal/game/cheat.go`, keys 1-9 on the menu in
  `cmd/play`). It hooks the engine at one point, right after
  `newGameVars` in state 8, and sets `RAM[$05C]` (stage index 0-8),
  `RAM[$051]` (stage number) and `RAM[$05D]` (stage colour), as the stage
  advance at `$741A-$743B` would. It is inactive unless triggered, so the
  traces still match.

## Stages

Nine layouts (the stage list table at `$4E49`, indexed by `RAM[$05C]`;
entry 9 repeats layout 0). Clearing the stage with index 8 makes
`RAM[$05C]` 9, which selects the ending: the `$77C5` castle data and
"CONGRATULATIONS". The stage colour is `$77` for index 0 and 9 and up,
else `tab_745a[index & 7]` (index 4 and 8 give `$11`: night). From the
code, unverified in play: after the ending `$757C` reloads `RAM[$054-$060]`
from `$51F3`, which resets the stage index to 0 but not the stage number,
so the layouts loop while the number keeps counting.

## Corrections

- 2026-10-10: the font was first described as `$45EB` +$180. The copy
  length is $180, but the glyph data proper is 43 characters. The rest is
  the text table at `$4743`.

- 2026-10-10: the jump-table trampoline is at `$4061`, not `$405F` as first
  written in CLAUDE.md. `$405F` holds the last two bytes of `JP $0047`.
