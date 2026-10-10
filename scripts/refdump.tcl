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
# Every LD A,R result is logged to <out>.rnd as (u32 tick, u16 address of
# the LD A,R, u8 value), so cmd/refcompare can feed the port the same
# "random" values; the R register itself cannot be reproduced natively.
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
set ::rh [open "$::out.rnd" w]
fconfigure $::rh -translation binary

proc refdump_rnd {pc} {
  puts -nonewline $::rh [binary format isc [expr {$::tick - 1}] $pc [reg A]]
}

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
  if {$::tick >= $::nticks} { close $::fh; close $::rh; exit }
}

# $403B: H.TIMI handler entry (every interrupt, sound runs here too).
# $404B: CALL $4555, the first instruction of a game tick after the busy
# guard at $4042-$4049.
debug set_bp 0x403B {} refdump_irq
debug set_bp 0x404B {} refdump_tick

# The instruction after each LD A,R (ED 5F) site: A holds the value read.
# Tick numbers in the log are those of the tick that is running.
foreach pc {0x686F 0x6C42 0x704B 0x7139 0x71E6 0x7227 0x725C 0x728C 0x72FB} {
  debug set_bp [expr {$pc + 2}] {} [list refdump_rnd $pc]
}
