# openMSX script: record the original game's state at the start of every
# game tick, as a reference for cmd/refcompare. It runs the real cartridge
# in an emulator purely as an offline oracle; the port executes no Z80 code.
#
#   openmsx -machine C-BIOS_MSX1_JP -cart <rom> -script scripts/refdump.tcl
#
# Environment: REFDUMP_OUT (output file, default ref.trace),
# REFDUMP_TICKS (number of ticks, default 3000), REFDUMP_KEYS (optional
# input schedule: "tick:row:mask:down|up ..." for keymatrixdown/up).
#
# Record format (little-endian), one per tick:
#   "TICK" u32 tick, u32 interrupts-so-far, RAM $E000-$E3FF (1024),
#   VRAM (16384), VDP R0-R7 (8)
# The output is ROM-derived: keep it outside the repository.

set throttle off
set ::out    [expr {[info exists ::env(REFDUMP_OUT)]   ? $::env(REFDUMP_OUT)   : "ref.trace"}]
set ::nticks [expr {[info exists ::env(REFDUMP_TICKS)] ? $::env(REFDUMP_TICKS) : 3000}]
set ::keys   [expr {[info exists ::env(REFDUMP_KEYS)]  ? $::env(REFDUMP_KEYS)  : ""}]
set ::tick 0
set ::irq 0
set ::fh [open $::out w]
fconfigure $::fh -translation binary

proc refdump_irq {} { incr ::irq }

proc refdump_tick {} {
  foreach k $::keys {
    lassign [split $k :] t row mask dir
    if {$t == $::tick} {
      if {$dir eq "down"} { keymatrixdown $row $mask } else { keymatrixup $row $mask }
    }
  }
  puts -nonewline $::fh "TICK"
  puts -nonewline $::fh [binary format ii $::tick $::irq]
  puts -nonewline $::fh [debug read_block memory 0xE000 0x400]
  puts -nonewline $::fh [debug read_block VRAM 0 0x4000]
  set s ""
  for {set i 0} {$i < 8} {incr i} { append s [binary format c [debug read "VDP regs" $i]] }
  puts -nonewline $::fh $s
  incr ::tick
  if {$::tick >= $::nticks} { close $::fh; exit }
}

# $403B: H.TIMI handler entry (every interrupt, sound runs here too).
# $404B: CALL $4555, the first instruction of a game tick after the busy
# guard at $4042-$4049.
debug set_bp 0x403B {} refdump_irq
debug set_bp 0x404B {} refdump_tick
