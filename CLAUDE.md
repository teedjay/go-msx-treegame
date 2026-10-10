# CLAUDE.md — native Go port of a 16KB MSX1 cartridge (tree-climbing game)

This project ports one specific MSX1 cartridge ROM to native Go. It follows
`NEXT-ROM-PORT-PLAN.md`, which was distilled from an earlier, completed port.
Every rule here exists because something went wrong without it. Read that
file for the war stories behind the rules. This file is the
project-specific, binding version.

**The goal is a native Go port, not an emulator.** No Z80 code runs at
runtime. Each routine is read, understood, and rewritten as ordinary Go
against a small hardware-abstraction interface. That choice shapes the legal
position, the verification strategy, and what "done" means for a routine.

---

## The ROM (source of truth)

| | |
| --- | --- |
| File (user must supply it; never committed) | `Magical Tree - Konami (1984) [RC-713] [1393].rom` |
| Size | 16384 bytes (16KB, mapped at `$4000-$7FFF`, no mapper) |
| CRC32 | `d4d30bcd` |
| MD5 | `ae7c3a41d9b23e7c889fb318203a43d9` |
| SHA-1 | `f4b58fecbcbc8cd307c34806ecece2a204c26c65` |
| SHA-256 | `b757aec09d839b4f8aba078ceda9b9809a8de8dc34f3df375d157fbe0dd8116c` |

Every address in the notes refers to this exact dump. `cmd/extract` must warn
when the checksum does not match.

The title and publisher may appear only where they are technically needed:
the ROM filename above, and the tracing notes. They must not appear in the
repo name, module path, window title, README headings or package names. The
working name is the generic **`go-msx-treegame`**.

### Facts established so far (2026-10-10, from a first byte-level look)

Confidence key: **High** = traced instruction by instruction and/or visually
confirmed. **Medium** = control flow clear, meaning inferred. **Low** =
plausible guess.

- **Header** (High): `41 42` "AB", INIT = `$408E`, STATEMENT/DEVICE/TEXT = 0.
- **Boot at `$408E`** (High): `DI` / `IM 1`, then installs `JP $403B` into
  `H.TIMI` (`$FD9A`), sets `SP=$E400`, and zeroes `$E000-$E3FF` with `LDIR`.
  It sets `$E005=1`, calls `$4507` (VDP init), clears `$E005`, calls BIOS
  `$013E` and `EI`. It then idles forever in `JR $` at `$40BC`. **The whole
  game runs from the VBlank hook.** There is no main loop outside the
  interrupt.
- **VBlank handler at `$403B`** (Medium): `DI`, BIOS `$013E` (reads VDP
  status), `CALL $7AFB`, then uses `$E005` bit 0 as a re-entrancy/busy flag.
  It increments `$E005` and continues to `CALL $4555` and `CALL $40C6` with
  interrupts enabled. This is the per-frame tick. Trace it first.
- **Game RAM** (High): `$E000-$E3FF`, stack top `$E400`. Address it as
  `c.RAM[0xNNN]` for `$ENNN` until a byte's role is confirmed.
- **VDP init at `$4507`** (High): calls `$7AF2`/`$40BE`, then `$4459` with
  `DE=$0000, BC=$4000, A=0` (looks like a 16KB VRAM clear; read `$4459` to
  confirm). It writes registers R0-R7 from the table at **`$452C`** through
  BIOS `WRTVDP` (`$0047`).
- **VDP ports** come from BIOS `$0006`/`$0007` (read at `$44F3`, `$4501`),
  not hard-coded `$98`/`$99`. No `OTIR`/`OUTI` found in a quick scan, so VRAM
  writes probably go through `OUT (C)` in helper routines near `$44F0`.
- **Computed-jump trampoline at `$405F`** (Medium): `ADD A,A` / `POP HL` /
  … / `LD E,(HL); INC HL; LD D,(HL); EX DE,HL; JP (HL)`. It is a jump table
  laid out inline after the `CALL`. **The disassembler must handle this**
  (Phase 1): it must decode the table entries that follow each call site as
  data and treat them as code targets.
- **BIOS usage seen**: `WRTVDP` ×2, `RDVRM` ×1, `WRTPSG` ×6, `RDPSG` ×1,
  `SNSMAT` ×2, plus `$013E` (VDP status read). Input appears to go through
  `SNSMAT` (keyboard matrix) and PSG port reads (joystick). Confirm both.

### VRAM layout (High: derived from the register table at `$452C`)

`R0..R7 = 02 E2 0E 7F 07 76 03 E4`

| Reg | Value | Meaning |
| --- | --- | --- |
| R0 | `$02` | Graphic 2 (Screen 2) |
| R1 | `$E2` | 16K, display on, VBlank IRQ on, **16×16 sprites**, no magnify |
| R2 | `$0E` | Name table **`$3800`** |
| R3 | `$7F` | Colour table **`$0000`** (bit 7 clear), full 3-thirds mask |
| R4 | `$07` | Pattern generator **`$2000`** (bit 2 set), full mask |
| R5 | `$76` | Sprite attribute table **`$3B00`** |
| R6 | `$03` | Sprite pattern table **`$1800`** |
| R7 | `$E4` | Text colour grey, **backdrop dark blue** |

This is **not** the MSX BASIC default. Colour data sits at `$0000` and
patterns at `$2000`, the reverse of the textbook layout. Any asset labelled
"pattern" or "colour" from a VRAM address must be checked against this
table. Before trusting the layout, **confirm it by compositing one
pattern+colour pair** into readable text or a recognisable shape.

### Still unknown (fill in as you go; move answers into the analysis doc)

- Graphics compression format and decompressor entry. Find the
  `LD DE,<src>` / `LD HL,<src>` setups that feed a common call. Do not assume
  it matches the previous project's RLE.
- Top-level state machine (attract → title → play → death → game over)
  dispatched from `$40C6`/`$4555`.
- Sound driver (the `WRTPSG` callers, probably near `$453B`).
- RNG source (look for `LD A,R`).

---

## Phase 0: repo hygiene and legal, before any code

1. **The first commit `.gitignore`s** `*.rom`, `assets/**/*.bin` and
   `docs/listing.asm`. All three are the rights holder's data.
2. **No ROM bytes in committed Go source.** A table longer than a few bytes
   goes through `cmd/extract` and is loaded via `//go:embed`. Never paste it
   in as a `[N]byte{…}` literal. Short constants such as a 2-byte tile pair
   or a palette index are fine. When in doubt, extract.
3. **The repo, module path and window title stay generic** (see above).
4. **Add `NOTICE` and a README "Legal" section up front.** Say the project
   is unofficial and non-commercial, and not affiliated with or endorsed by
   the rights holder. Say what is and is not distributed, and that the
   engine executes no original code. Invite rights holders to open an issue.
   Add a `LICENSE` if the README calls the project open source.
5. **Module path:** `go-msx-treegame` (local-only repo for now, no remote).

Toolchain available here: Go 1.27 (`/opt/homebrew/bin/go`) and openMSX
(`/opt/homebrew/bin/openmsx`). Use openMSX only to *observe* the original:
reference screenshots and checking behaviour. It never runs inside the port.

## Phase 1: the tooling spine (build before porting)

| Tool | Why it has to exist |
| --- | --- |
| `internal/z80dis` + `cmd/disasm` | Recursive-descent disassembler with **byte-level coverage tracking**. It must handle the inline jump-table trampoline at `$405F`, and it starts from `$408E` (INIT) and `$403B` (H.TIMI handler). |
| `cmd/extract` | Rebuilds every asset from the ROM via a **manifest** (address, length, and method: raw, compressed or derived). Has a `-check` mode that re-derives everything and diffs without writing. Warns on a checksum mismatch. |
| `internal/<compression>` + decoder cmd | Once the format is traced. Decodes to flat buffers. |
| `cmd/gfxpreview`, `cmd/gfxviewer` | Render byte ranges as tile grids to classify data. |
| `internal/msxrender` + `cmd/screenshot` | **The most valuable tool.** See below. |
| `cmd/sndpreview` | Only once you reach audio. |

### One renderer, shared, with a headless way to use it

Ebitengine cannot open a window from a sandboxed shell. **Never** write a
throwaway compositor inside a test. It drifts from the real renderer, and
then you end up debugging differences that do not exist in the real program.

- `internal/msxrender` takes VRAM in (with the layout above) and returns an
  `image.RGBA`.
- The game draws every frame through it.
- Headless `cmd/screenshot` drives a real console instance (attract →
  title → into play, holding the accelerator) and writes PNGs through *the
  same* package. These stills are the regression baseline.

## Phase 2: map the ROM before porting

1. Generate `docs/listing.asm` (gitignored). Re-run `cmd/disasm` whenever
   new code addresses are confirmed.
2. The checksums are recorded above. Copy them into the analysis doc too.
3. The VRAM layout is derived (above). Confirm it visually.
4. Map the state machine hanging off the VBlank tick (`$403B` → `$4555`,
   `$40C6`) before porting leaf routines.

## Phase 3: assets

- **Check whether a block is compressed before extracting it.** Grep the
  listing for a source pointer feeding a call into the decompressor. A raw
  render that "looks roughly like tiles" does not confirm anything.
- **Render the entire decoded output** and check that it is a complete,
  coherent set. A spot check is not enough.
- **Tight packing is the strongest cheap signal.** A correctly sized
  extraction ends exactly where the next known thing begins.
- **Verify provenance empirically; never trust a filename.** A ROM address
  can decode "successfully" into plausible, completely wrong data.
- Assets computed at load time (mirrored tiles, derived colours) are
  **derived** manifest entries, not captured blobs.

## Phase 4: port the engine

Work outward from the VBlank tick, one routine at a time.

### Conventions (strict)

- Every ported function's doc comment names its source range:
  `// Ported from $4685-$46A8`.
- State confidence inline (High / Medium / Low, as defined above). A
  confident function name must not claim more than the tracing supports.
- **Prefer an honest stub, `// TODO: not yet ported, see $XXXX`,** over a
  guessed implementation.
- Unconfirmed RAM stays `c.RAM[0x0NN]` for `$E0NN`. Promote it to a named
  accessor only once its role is confirmed, and note what confirmed it.
- The analysis doc is append-only for corrections. Add a correction note;
  never edit history away.

### The failure mode that costs the most: assuming routine boundaries

1. **Read from the actual branch target**, not from the nearest label.
2. **Follow every fall-through.** A `RET` is not the end if a conditional
   branch above it skips past to more code.
3. **Read the callee before modelling a call.** Never infer a shared
   routine's behaviour from one call site. Screen 2 helpers that write three
   copies 2KB apart, one per screen third, are the classic trap.
4. **In loops, the jump target defines the semantics.** For example, a
   `DJNZ` back to a byte-read means repeat, while one that skips the read
   means literal.
5. When a bug makes no sense, re-read the routine's raw bytes, not your Go.

### Z80 / MSX semantics to model exactly

- `DJNZ` decrements `B`, not RAM. A per-frame `LD B,(state)` + `DJNZ` chain
  is a switch, not a countdown.
- 8-bit wrap is load-bearing. Do not widen before `ADD A,A` and similar.
- Scores are almost certainly packed BCD (`DAA`). Use one
  `bcdAddCarry(a, b, carryIn)` helper everywhere.
- 16-bit loads and stores touch two bytes, and overlapping stores can touch
  three.
- TMS9918A: colour 0 is transparent and shows the backdrop (dark blue here,
  not black). Sprite Y `$D0` ends the sprite list. The displayed row is Y+1.
  With 16×16 sprites the pattern number's low 2 bits are ignored and the
  number counts in 8-byte units.
- 16×16 layouts differ: hardware sprites are TL/BL/TR/BR, while "big"
  background characters are usually TL/TR/BL/BR. Justify the layout from
  the destination table, never from "it rendered as something".
- `LD A,R` entropy: substitute a real RNG, preserve the outcome
  distribution, and document the deviation.
- The original runs everything inside the VBlank interrupt, with
  re-entrancy guarded by `$E005`. In the port, model this as one
  deterministic `Tick()` per frame. Document any case where the original
  could overrun a frame and what the port does instead.

## Phase 5: make it playable, then keep it that way

Before every commit:

```bash
go build ./... && go vet ./...
go run ./cmd/extract -check
go run ./cmd/screenshot -out /tmp/shots && \
  for f in screenshots/*.png; do cmp "$f" "/tmp/shots/$(basename "$f")"; done
```

- Run `-check` after touching the manifest, the assets package, or how any
  asset is produced.
- Run the screenshot byte-diff after any engine change that should not
  alter behaviour. When a change *is* meant to alter what is drawn,
  regenerate the stills, commit them, and say so in the commit message.
- **Commit after each logical change.** Never commit ROM-derived files. If
  `git status` offers them, `.gitignore` is broken.
- Write throwaway probes as real programs (scratch `cmd/` or temporary
  `_test.go`) and delete them before committing. Report measurements, not
  impressions.

## Phase 6: only once it plays

Extensions sit *over* the port, never inside it. They read the VRAM and
state the engine already produces and leave the engine untouched.

---

## Working with the user

- **The user playing the game is the best bug detector.** Ask them to play
  after each milestone.
- **When the user says it's still wrong, believe them.** Ask for a
  real-hardware (or openMSX) screenshot or video whenever behaviour is
  disputed. One is worth a day of tracing.
- Report measurements ("all N stills byte-identical"), not "looks right".
- Say out loud whether something is a port bug or a faithful reproduction
  of an original quirk, and why.

## Documentation hygiene

- Keep one analysis doc (`docs/ANALYSIS.md`) with:
  - an **Open issues** list at the top, holding only open items
  - the checksums
  - the confidence-rated asset inventory
  - the VRAM map
  - the compression format
  - dated write-ups of every bug and correction
- Sweep periodically for stale claims: "not yet ported" comments on ported
  code, outdated layout descriptions, and instructions that are no longer
  true.
- When an open issue is resolved, move its write-up into the body. Do not
  delete it.

## Suggested first week

1. Repo, `.gitignore`, `NOTICE`, README skeleton, module path. *(Phase 0)*
2. Disassembler (with the `$405F` trampoline) + listing. *(Phases 1-2)*
3. Confirm the `$452C`-derived VRAM layout by compositing one
   pattern+colour pair.
4. Trace the compression format, write the decoder, extract the first
   assets and **render them fully**.
5. `cmd/extract` with manifest and `-check`; `internal/msxrender`;
   `cmd/screenshot`.
6. Port the boot path (`$408E`) and VBlank tick (`$403B`) far enough to
   show the title screen. Commit the first screenshot as the baseline.
