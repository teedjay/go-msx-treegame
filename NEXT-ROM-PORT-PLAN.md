# Porting an MSX cartridge ROM to native Go: project plan

Intended as the `CLAUDE.md` for a new ROM-port project. It is distilled from
two completed ones (both 16KB MSX1 cartridges, fully ported and playable).
Every rule below exists because something went wrong without it, or because
it is the thing that actually caught the bug. Where a rule cost real rework,
the story is kept: a rule with its scar attached gets followed.

Replace the bracketed bits with this project's specifics as you learn them.

**The goal is a native Go port, not an emulator.** No Z80 code runs at
runtime. Each routine is read, understood, and rewritten as ordinary Go
against a small hardware-abstraction interface. That distinction drives
everything else: the legal position, the verification strategy, and what
"done" means for a routine.

**The second port's headline lesson: verify against the original tick by
tick, from the first ported routine.** An emulator (openMSX with the free
C-BIOS) used *offline* as a reference oracle records the original's state at
every game tick, and a comparator runs the port in lockstep against it. In
the second project this reached a byte-for-byte match over tens of thousands
of ticks of real play. It found bugs that static reading and screenshots
never would have, and it named each one's address. See Phase 1, "The
reference oracle", and Phase 5.

---

## Phase 0: repo hygiene and legal, before any code

Do this on day one. The first project retrofitted it at the end, and it cost
a full day of migration plus a careful audit, because by then ~3,000 bytes of
ROM data had accumulated inside committed Go source.

1. **`.gitignore` the ROM, the extracted assets and the generated
   disassembly in the very first commit:** `*.rom`, `assets/**/*.bin`,
   `docs/listing.asm`. All three are the rights holder's data, decompressed
   or decoded but not yours. Reference traces and VRAM dumps are ROM-derived
   too, so keep them outside the repo (for example `/tmp`). Screenshots of
   your own running engine are fine.
2. **No ROM bytes in committed Go source.** This is the rule that is easiest
   to break by accident and most expensive to undo. A data table longer than
   a few bytes goes through the extract tool (Phase 1) and is reached via
   `//go:embed`, not pasted in as `[217]byte{0x..., ...}`. Short constants
   (a 2-byte tile pair, a palette index, an 8-byte VDP register table) are
   fine; use judgement, but when in doubt extract it.
3. **Name the repo for what it is, not what it plays.** Use a generic,
   descriptive name. Do not put the original title or publisher in the
   repository name, the module path, the window title, or any heading.
   Reference them only where technically necessary: the ROM filename a user
   must actually look for, and the tracing notes.
4. **Add `NOTICE` and a README "Legal" section up front.** Say the project
   is unofficial and non-commercial, and not affiliated with or endorsed by
   the rights holder. Say what is and is not distributed, and that the
   engine executes no original code. Invite rights holders to open an issue.
   Add a `LICENSE` too if the README is going to call the project open
   source.
5. **Decide the module path now.** Renaming later is mechanical but touches
   every import. A local-only repo can simply use the bare repo name.

## Phase 1: the tooling spine

Build these before porting anything. Each one paid for itself many times
over. Roughly in this order:

| Tool | Why it has to exist |
| --- | --- |
| `internal/z80dis` + `cmd/disasm` | Recursive-descent disassembler with **byte-level coverage tracking**, so a misaligned decode can't quietly corrupt already-confirmed code. Needs special handling for computed-jump trampolines; find them early. Give it `-gaps` (unclaimed ranges *with the instructions that reference into each*) and `-peek A-B` (linear decode without claiming). |
| `cmd/extract` | Rebuilds every asset from the ROM via a **manifest** (address, length, and method: raw range, compressed stream, or derived). This is the thing that lets you ship no ROM data. Give it a `-check` mode that re-derives everything and diffs without writing. |
| `internal/<compression>` + decoder cmd | The graphics may well be compressed. Decode to flat buffers for extraction. **Check each block**: in the second project the font was stored raw while everything around it was RLE. |
| `cmd/gfxpreview`, `cmd/vramview` | Render byte ranges or raw VRAM dumps as tile and sprite sheets, through the shared renderer. |
| `internal/msxrender` + `cmd/screenshot` | **The most valuable visual tool.** See "One renderer" below. |
| `scripts/refdump.tcl` + `cmd/refcompare` | **The most valuable tool overall.** See "The reference oracle" below. Build it as soon as the boot path is ported. |
| `cmd/sndpreview` | Once you reach audio: render a sound id to WAV through the ported sequencer and a PSG model, so you can check sounds without a window. |

### Disassembler lessons from the second project

- **Grow inline jump tables one entry at a time.** For a `CALL trampoline`
  followed by `DW` entries, add a single entry, run the full descent to
  completion, then try the next. Code discovered from the earlier entries
  then stops the table from overrunning into it. Report such tables as
  "heuristic" until you confirm the length from tight packing (the table
  ends exactly at its first target) or from the index range.
- **Treat `LD rr,nn` / `PUSH rr` as a code reference.** The pushed value is a
  continuation that a later `RET` jumps to. Both top-level dispatchers in the
  second ROM pushed a continuation before dispatching, so **every handler's
  `RET` landed in a third routine, not back at the caller**. Missing this
  would have left two whole routines unported. It is another disguise of the
  routine-boundary trap in Phase 4.
- **Callbacks passed in a register** (`LD DE,routine` then a helper doing
  `EX DE,HL / JP (HL)`) are invisible to call-following. Search for the
  helper, list the values loaded into DE before it, and add them as entries.
- **Format immediates and call targets separately.** `LD DE,$0000` printed
  as `LD DE,CHKRAM` (a BIOS label) misleads. Only call and jump targets get
  BIOS names; memory operands get system-variable names.
- A "data reference" in the gap report may be a VRAM address the VDP masks
  (`LD DE,$7800` meant `$3800`) feeding a fall-through into a fill routine.
  Read the code around a surprising reference before classifying the target.

### One renderer, shared, with a headless way to use it

Ebitengine (or any windowed toolkit) may not run in a sandboxed shell, which
means you cannot see what you just built. The trap is to write a throwaway
compositor inside a test to inspect VRAM. **Do not.** That copy silently
drifts from what the game actually renders, and then you end up debugging a
rendering difference that does not exist in the real program.

Instead:

- Put the compositor in its own package (`internal/msxrender`): VRAM in,
  `image.RGBA` out.
- The game draws every frame through it.
- A headless `cmd/screenshot` drives a real console instance (attract
  sequence → menu → into a stage, holding the accelerator) and writes PNGs
  through *the same* package.

A still then cannot show something the running game would not. This also
gives you a regression test in Phase 5.

### The reference oracle (lockstep traces)

The original runs in openMSX purely as an offline reference. The port never
executes Z80 code. Set it up as soon as the boot path is ported:

- **Record at the start of every game tick.** For a VBlank-driven game, put
  a breakpoint on the first instruction of the tick body (after any
  re-entrancy guard). Dump game RAM, all 16KB of VRAM, VDP registers R0-R7,
  and the interrupt count so far. Use the Tcl `debug read_block` and
  `debug read "VDP regs"` commands. A breakpoint that has a command does not
  pause emulation.
- **Compare in lockstep.** `cmd/refcompare` boots the port and, for each
  recorded tick, runs the per-interrupt work (sound) as many times as the
  reference took interrupts. It compares RAM, VRAM and registers, runs the
  tick, and stops at the **first** differing byte, with ref/port PNGs of
  that tick. The first differing address almost always points at the
  routine that owns it. Exclude the Z80 stack area of RAM.
- **Prove the comparator can fail.** Flip one byte in a copy of a trace and
  check that it is reported at the right tick and address. A comparator that
  always passes is worse than none.
- **Feed input by tick.** The trace script takes a schedule
  (`tick:row:mask:down|up` for `keymatrixdown/up`), applied at the tick-start
  breakpoint, before the game reads input. The comparator replays the same
  schedule into the port. Use the keyboard modes for traced sessions: the
  key matrix is easy to drive and the joystick is not. Give the playable
  frontend a `-record` flag that writes this format, so a human's session
  can be replayed into the original and diffed.
- **Script games for coverage.** A long pseudo-random input schedule
  started from the menu covered most player states in the second project.
  Measure coverage from the traces (which state values occurred), and list
  what has not been exercised as an open issue. Rare paths (stage clear,
  game over, 2 players) need a human.
- **When a byte differs and you cannot see why, ask the oracle who wrote
  it.** `debug set_watchpoint write_mem <addr>` with a command that logs
  `[reg PC]` and `$::wp_last_value`. In the second project, `$E130` differed
  after stage setup. The watchpoint named an `RR (IX+0)` in a load-time
  sprite-mirroring loop that used RAM as a scratch buffer, and one byte of
  that buffer survived later initialisation. The port had only reproduced
  the loop's VRAM result. **Derived assets can have RAM side effects: port
  those too.**
- openMSX practicalities: C-BIOS machines ship with it (`C-BIOS_MSX1_JP` for
  60Hz). `screenshot` with `set throttle off` gave black images, so dump VRAM
  and render it with your own renderer instead, which is better anyway.
  macOS has no `timeout`, so run openMSX in the background with a watchdog
  loop. Several instances can record in parallel.

## Phase 2: map the ROM before porting

1. Generate the listing. Re-run `cmd/disasm` whenever you confirm new code
   addresses: they feed back into its jump-table heuristics and recover more
   of the ROM automatically.
2. Record the ROM's checksums (CRC32/MD5/SHA-1/SHA-256) and size in the
   analysis doc, and have every tool warn on a mismatch. Every address in
   your notes is meaningless against a different dump.
3. **Derive the VRAM layout from the ROM's own VDP register writes.** Do not
   assume the MSX BASIC default. Both projects used the same non-default
   layout (colour table at `$0000`, patterns at `$2000`, name table at
   `$3800`, sprites at `$1800`/`$3B00`). Assuming the textbook layout
   mislabelled every pattern/colour pair backwards in the first project:
   "pattern" files that were colour data and vice versa. Confirm the
   derivation by compositing one pattern+colour pair and checking you get
   readable text or a recognisable shape.
4. Find the top-level state machine and the per-frame tick early. Everything
   else hangs off them. Check whether the game runs entirely inside the
   VBlank interrupt (INIT ending in `JR $`) and whether it has a busy flag:
   then some ticks take several frames (see Phase 5, "Tick pacing").
5. **Spare VRAM is working storage.** The second ROM kept its stage map in
   VRAM above the sprite attribute table and read it back through BIOS
   `RDVRM`. Do not assume VRAM holds only display data.

## Phase 3: assets

- **Check whether a block is compressed before extracting it.** Grep the
  listing for a `LD DE,<addr>` (or `LD HL,<addr>`) feeding a call into the
  decompressor. A raw render that "looks roughly like tiles" is not
  confirmation; that is exactly how a font shipped with subtly corrupted
  glyphs.
- **Render the entire decoded output and check it is a complete, coherent
  set.** Not a spot check. "It terminates cleanly" and "most tiles look
  fine" were both satisfied by a decoder with its two run kinds swapped.
- **Tight packing is the strongest cheap signal**: a correctly sized
  extraction ends exactly where the next known thing begins. In the second
  project every stream ended exactly at the next stream or at code.
- **Verify provenance empirically; never trust a filename.** In the first
  project two assets were *named* for a ROM address that really is a valid
  compressed stream. Decoding from it succeeds and yields plausible,
  completely wrong data. They were actually load-time mirrors of slices of
  other assets. Confirm each manifest entry by extracting it and diffing
  against the file you already have.
- Assets the original computes at load time (mirrored tiles, derived
  colours) belong in the manifest as derived entries, computed from their
  source asset, not as separate captured blobs.
- **Store multi-segment streams as flat segments, with the VRAM addresses in
  the manifest.** Have the extractor fail if a decode does not produce
  exactly the declared segments. The manifest then doubles as a checked
  record.
- **Register every data gap the code references as a Raw entry**, even small
  tables, and give the engine one `romByte(addr)` lookup that serves any
  address inside a Raw entry and **panics on anything else**. Routines that
  walk pointer tables can then be ported with their original address
  arithmetic. A read of something never extracted fails loudly instead of
  returning a plausible zero. In the second project that panic is what
  turned a silent run-off-the-map bug into an obvious crash with an address.
- Overlaps are allowed when the original does them. The second ROM copied
  `$180` bytes of font into VRAM, the last `$28` of which were its text
  table. The asset keeps the full length; the text is a separate entry.
- Runtime-decoded data stays runtime-decoded. Object shapes whose rows were
  RLE streams read at draw time are ported as the decoder routine reading
  raw ROM data, not pre-decoded.

## Phase 4: port the engine

Work outward from the state machine, one routine at a time, running the
lockstep comparison as you go.

### Conventions (keep these strict)

- Every ported function's doc comment names the Z80 address range it came
  from (`// Ported from $4685-$46A8`), so anyone can jump back to the
  listing and check the translation.
- **Say what is confirmed vs inferred, inline.** Use a three-level key: High
  (traced instruction by instruction, and/or visually or trace confirmed),
  Medium (control flow clear, meaning inferred), Low (plausible guess). A
  confident-sounding function name must not imply more certainty than the
  tracing supports. When the role is unclear, name the function by its
  address (`upd6B53`) and put the guess in the comment.
- **Prefer an honest stub with `// TODO: not yet ported, see $XXXX`, which
  sets a `Console.Unported` marker, over a guessed implementation.** The
  comparator then reports "ticks 0-N match, tick N reached unported $XXXX".
  That is a clean progress measure.
- Address unconfirmed RAM the way the original did (`c.RAM[0x0NN]` for
  `$E0NN`). Promote it to a named accessor only once the byte's role is
  confirmed, and note what confirmed it.
- **Keep CPU-address helpers** (`rd`, `wr`, `rd16`, `ldir`, `lddr`, `inc`,
  `dec` on 16-bit addresses) so pointer-walking routines translate line for
  line. `wr` to the cartridge range is a no-op, as on hardware. The second
  ROM wrote to its own ROM in three places, all development leftovers with
  no effect, and the port must not "fix" them. `wr` to an unmapped address
  panics.
- Keep the analysis doc append-only for corrections: add a correction note
  rather than editing history away. The record of *how* something was got
  wrong is worth as much as the right answer.

### The one failure mode that will cost you the most

**Assuming where a routine starts and ends, instead of reading it.**

This single mistake, in many disguises, caused nearly every real bug in the
first port, and kept turning up in the second:

- A shared helper was a *tripler*: it wrote three copies, 2KB apart, for the
  three Screen 2 thirds. It was read as a single write because the call site
  looked like one.
- A terminator byte in a data stream was really a *gap marker*; the stream
  continued past it.
- A "row repeat" copier actually advanced its source pointer each row, so
  rows walked forward through ROM.
- `JP L70E5` landed in the *middle* of another routine, not at the start of
  the one whose `CALL` sat at that address. Control continued through three
  more calls that the port never ran.
- A scoring routine looked like it ended at its `RET`; the no-carry branch
  three instructions earlier fell through into the high-score update, which
  therefore never ran at all.
- A state handler was read starting three instructions too late, dropping
  the two `CALL`s that cleared the screen.
- *(Second project)* Dispatchers `PUSH`ed a continuation before jumping
  through a table, so each handler's `RET` went somewhere else entirely.
- *(Second project)* A screen-clear routine set up its arguments and fell
  through into the fill routine below it, with no `CALL` or `JP` to show it.

The rules that would have prevented them:

1. **Read from the actual branch target, not from the nearest label.** When
   something jumps to `$411A`, start reading at `$411A`.
2. **Follow every fall-through.** A `RET` is not the end of a routine if a
   conditional branch above it skips past to more code. Trace each exit, and
   check whether the routine's last instruction is a jump at all.
3. **Read the callee before modelling a call.** Never infer what a shared
   routine does from how one call site uses it.
4. **For loops, the jump target tells you the semantics.** In the first ROM
   the two compression run-kinds are distinguished *only* by whether `DJNZ`
   targets the byte-read or the instruction after it. One re-emits without
   advancing (repeat), the other advances (literal). The control-byte ranges
   alone look symmetric and will mislead you.
5. **Track the stack.** Any `PUSH` of a constant, `POP` of a return address
   or `EX (SP),HL` changes where control goes.
6. When a bug makes no sense, re-read the raw bytes of the routine rather
   than re-reading your Go, or ask the oracle (a watchpoint) who wrote the
   byte.

### Z80 and MSX semantics that bit us

- **`DJNZ` decrements a register, not RAM.** A dispatch that loads a state
  byte into `B` each frame and `DJNZ`es on it is an *inefficient switch over
  the current value*, not a persistent countdown. Modelling it as a RAM
  countdown broke the main loop for a long time.
- **8-bit wrap is load-bearing.** `ADD A,A` wraps at 8 bits, and widening to
  `uint16` before doubling changes behaviour. Model the width the original
  used: `3*A` index arithmetic is 8-bit, `L+5` without carry into H is
  low-byte only, and `ADD HL,BC` does carry.
- **BCD via `DAA`**: scores are packed BCD. Model `DAA` exactly (the
  correction from the H and C flags and the result, for both add and
  subtract) rather than writing a "BCD add" that assumes valid operands.
  `binary → BCD` loops (`ADD HL,HL` / `ADC A,A` / `DAA`) depend on it.
- **16-bit loads read two bytes** (`LD BC,(E000h)` fetches `$E000` *and*
  `$E001`), and overlapping 16-bit stores can touch three addresses (the
  second ROM set a hi-score of 999999 with two overlapping stores).
- **Chained `SBC HL` carries a borrow.** Two `SBC HL,rr` in a row feed the
  first borrow into the second. In the second ROM that meant ending a jump
  *higher* than it started counted as a long fall.
- **Flags survive across instructions that don't touch them**, and `EX
  AF,AF'` can carry a comparison result through a whole loop. Follow which
  instruction last set the flag that a branch tests.
- **Stale registers.** Occasionally a branch tests a register the routine
  only sets on some paths (`BIT 2,E` where E was loaded only if the enemy
  moved). Do not invent a value silently. Pick the most plausible one, mark
  it Medium in the code, list it as an open issue, and let a trace that
  exercises that path decide.
- **TMS9918A specifics:** colour index 0 is transparent and shows the
  backdrop (not black). Sprite Y `$D0` terminates the sprite list for all
  following entries. The displayed sprite row is Y+1. A 16x16 sprite's
  pattern number has its low 2 bits ignored and the table is addressed in
  8-byte units, not 32. At most four sprites show on one line.
- **The same 32 bytes have multiple 16x16 layouts in one ROM.** Hardware
  sprites are four 8x8 quadrants in TL/BL/TR/BR order; "big font"
  background characters are TL/TR/BL/BR; neither is 16 interleaved rows.
  All three produce *something* plausible from the same bytes. Justify the
  layout from the destination address and which hardware table it targets,
  never from "it rendered as something".
- **Entropy sources (`LD A,R`) have no equivalent in a native port.** Read
  the value through a pluggable source: a real RNG (values 0-127, since R's
  bit 7 stays 0 unless the game does `LD R,A`) in normal play, and in
  lockstep runs the values the trace script logged at each `LD A,R` site.
  Have the comparator flag a read at a different site or tick, because that
  alone pinpoints a control-flow difference. Document the deviation rather
  than faking the source.
- **Sound state is game state.** The game logic may read the sound driver's
  RAM (for example, waiting for a tune to end before changing state). Then
  the sequencer must be ported faithfully and early, long before you have
  audio output. Port it as register writes into a PSG register array, and
  add a separate chip model (tones, noise, mixer, volume table, envelope)
  later for output.
- **The BIOS port-number bytes** at `$0006`/`$0007` may be used swapped
  relative to the documentation. It is harmless on real machines, where both
  are `$98`; just don't let it confuse your VDP reading.

### Go pitfalls when transliterating

- **Operator precedence differs from what Z80 thinking suggests.** In Go,
  `&^`, `<<`, `>>`, `&` and `*` bind tighter than `|`, `^`, `+` and `-`,
  and `|` and `+` share a level. `col|0x08&^0x80` is `col|(0x08&^0x80)`,
  and `pc>>1|pc<<7+0x88` only worked by luck. Parenthesise every mixed
  bitwise/arithmetic expression. A review pass caught one real bug of
  exactly this kind in the second project.
- Byte-typed arithmetic wraps like the Z80 does; compute in `byte`, not
  `int`, unless you are deliberately testing for a borrow.

## Phase 5: make it playable, then keep it that way

Getting it on screen early is what surfaces bugs; static analysis does not.

**Verification, run before every commit:**

```bash
go build ./... && go vet ./... && go test ./...
go run ./cmd/extract -check                       # every asset still matches the ROM
go run ./cmd/refcompare -trace /tmp/attract.trace # port matches the original, tick by tick
go run ./cmd/refcompare -trace /tmp/play.trace -keys /tmp/play.keys
go run ./cmd/screenshot -out /tmp/shots && \
  for f in screenshots/*.png; do cmp "$f" "/tmp/shots/$(basename "$f")"; done
```

- Run `-check` after touching the manifest, the assets package, or how any
  asset is produced. A wrong address fails loudly instead of shipping
  plausible, wrong data.
- **Run `refcompare` after every engine change.** Keep at least one attract
  trace (no input) and a few scripted-input game traces. They must keep
  matching through every ported state. This is the strongest test you will
  have: it checks every byte of RAM and VRAM on every tick.
- Run the screenshot byte-diff after any engine change *not* meant to alter
  behaviour (a refactor, moving data into assets, a rename). It is the
  cheap smoke test when no trace is at hand. When a change *is* meant to
  alter what is drawn, regenerate the stills, commit them, and say so in the
  message.

**Porting order that worked:** title and attract first (verified tick by
tick), then the whole game tick at once, then fix diffs from the first
divergent byte. Porting a tick's subsystems one at a time and verifying each
in isolation does not work with whole-tick traces, because a partial tick
diverges immediately. Read everything the tick calls, port it all, and let
the comparator walk you through the mistakes. In the second project the
complete gameplay port needed one fix to match the 1700-tick demo.

**Tick pacing.** In a VBlank-driven game with a busy flag, a tick that takes
longer than a frame makes the following interrupts run sound only. The
trace's interrupt counts show this: in the second ROM a stage load spanned
106 frames and some gameplay ticks spanned 2. A port that runs one tick per
frame plays those moments faster. Decide deliberately how to model it, and
document the choice as an open issue until you do.

**Commit after each logical change**, once build, vet and tests are clean.
Never commit ROM-derived files. If `git status` offers them, `.gitignore` is
broken.

**Write throwaway probes as real programs, delete them after.** Use
temporary `_test.go` files in the engine package or a scratch `cmd/`
directory, and remove them before committing. Measuring a change ("off-road
object-frames fell from 175 to 20 over 12,000 frames") beats asserting it.

## Phase 6: only once it plays

Extensions (a second renderer, new modes, cheats, tooling) should sit *over*
the port, never inside it. The first project added a 3D chase-camera mode as
a separate package that reads the VRAM the engine already wrote and
reinterprets it. The engine itself was untouched, so the 3D mode shows the
real simulation and cannot desync from it. Keep that boundary.

When an extension must change game state (a level-select cheat):

- Put it in its own file, hook the engine at **one** documented point, and
  make it inactive unless triggered, so every trace still matches.
- **A shortcut skips state set-up that the normal path performs.** List every
  variable the skipped path establishes, not just the obvious ones. The
  second project's cheat set the stage index, number and colour. But the
  "bottom of map" marker is normally set when you finish the previous stage,
  so on a cheat-started stage a fall scrolled off the end of the map and
  crashed. Derive such values from the ROM data itself (here, the stage's
  first map record) rather than hard-coding them, and test the derivation
  against what the game actually builds.
- Reproduce a user-reported extension bug headlessly with a failing test
  first, then fix.

---

## Working with the user

- **The user playing the game is your best bug detector** for paths the
  traces have not covered. Ask them to play after each milestone, and offer
  `-record` so their session can be replayed into the original and diffed.
- **When the user says it's still wrong, believe them.** On one bug in the
  first project I twice concluded "this is genuine ROM behaviour" and was
  wrong both times; a real-hardware screenshot they supplied settled it.
  Both times the error was the same: combining two true statements into a
  false one without reading the routine they both depended on.
- **A real-hardware (or reference-emulator) screenshot or trace is worth a
  day of tracing.** With the oracle you can produce your own; do so whenever
  behaviour is disputed.
- **Report measurements, not impressions.** "All 12,000 ticks match", "all
  seven stills byte-identical" and "longest off-road stretch 86 frames → 2"
  are checkable; "looks right" is not. Say plainly which paths are verified
  and which are only translated.
- Distinguish, out loud, between a bug in the port and faithful reproduction
  of an original quirk. Both are common. Say which you think it is and why.

## Documentation hygiene

- Keep one analysis doc with:
  - an **Open issues** list at the top: what is actually open, including
    unverified paths and approximations, with nothing resolved left in it
  - the checksums
  - the state machine table, with what verified each state
  - the confidence-rated asset inventory
  - the VRAM map
  - the compression format
  - a verification log (which traces, how many ticks, what matched)
  - dated write-ups of every bug and correction
- **Sweep for stale claims periodically.** Doc comments rot fast. Near the
  end of the first project a sweep found four comments still saying "not
  yet ported" about work long since finished, an instruction telling future
  sessions to commit files that must never be committed, and a layout
  section describing a directory structure that had changed. Each had been
  true when written.
- When you move a resolved item out of the open list, move the *write-up*
  into the body rather than deleting it.

---

## Suggested first week

1. Repo, `.gitignore`, `NOTICE`, README skeleton, module path. *(Phase 0)*
2. Disassembler with coverage tracking, `-gaps` and `-peek`; listing; ROM
   checksums recorded. *(Phases 1-2)*
3. VRAM layout derived from the ROM's own register writes and confirmed by
   compositing one pattern+colour pair.
4. Compression format traced (if any); decoder; first assets extracted and
   **fully rendered** for inspection.
5. `cmd/extract` with a manifest and `-check`; `internal/msxrender`;
   `cmd/screenshot`. Nothing ROM-derived committed, ever.
6. Boot path, frame tick and state machine ported far enough to show the
   title screen. **Then `scripts/refdump.tcl` and `cmd/refcompare`**, with
   the attract sequence matching tick by tick. Commit the first screenshot
   as the regression baseline.
7. Read the whole game tick, port it, and drive the comparator to a match
   on the demo, then on scripted-input games. Then hand it to the user to
   play.
