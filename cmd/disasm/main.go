// Command disasm writes a recursive-descent disassembly of the cartridge
// with byte-level coverage tracking. A byte claimed by one decode is never
// re-decoded differently; overlapping claims are reported as conflicts.
//
// The listing contains ROM bytes and is gitignored. Never commit it.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-msx-treegame/internal/rom"
	"go-msx-treegame/internal/z80dis"
)

type cover uint8

const (
	covUnknown cover = iota
	covHead          // first byte of an instruction
	covBody          // later byte of an instruction
	covTable         // jump-table word
	covHeader        // cartridge header
)

type disasm struct {
	r      *rom.ROM
	cov    [rom.Size]cover
	insts  map[uint16]z80dis.Inst
	labels map[uint16]bool
	// xrefs records who jumps/calls each in-ROM target.
	xrefs     map[uint16][]uint16
	queue     []uint16
	conflicts []string
	indirect  []uint16
	// tableAt maps a table start to its entry count once laid down.
	tableAt map[uint16]int
	// growing are inline tables still being extended by the heuristic.
	growing   []*growTable
	heuristic map[uint16]bool
	confirmed map[uint16]int
}

type growTable struct {
	addr  uint16
	count int
	done  bool
}

func (d *disasm) idx(a uint16) int { return int(a) - rom.Base }

func (d *disasm) label(a uint16) string {
	if n, ok := names[a]; ok {
		return n
	}
	if rom.Contains(a) {
		return fmt.Sprintf("L%04X", a)
	}
	return fmt.Sprintf("$%04X", a)
}

// labeler names only addresses that will actually carry a label, so
// immediates that merely look like ROM addresses stay numeric.
func (d *disasm) labeler(a uint16) string {
	if _, ok := names[a]; ok {
		return names[a]
	}
	if rom.Contains(a) && d.labels[a] {
		return fmt.Sprintf("L%04X", a)
	}
	return fmt.Sprintf("$%04X", a)
}

// refLabeler formats immediates and memory operands: BIOS routine names are
// not applied (a constant $0000 is not CHKRAM), system variables are.
func (d *disasm) refLabeler(a uint16) string {
	if a < rom.Base {
		return fmt.Sprintf("$%04X", a)
	}
	return d.labeler(a)
}

func (d *disasm) addTarget(a, from uint16) {
	if !rom.Contains(a) {
		return
	}
	d.labels[a] = true
	d.xrefs[a] = append(d.xrefs[a], from)
	d.queue = append(d.queue, a)
}

// walk decodes linearly from pc until control flow stops.
func (d *disasm) walk(pc uint16) {
	var prevLoad z80dis.Inst
	for {
		if !rom.Contains(pc) {
			return
		}
		switch d.cov[d.idx(pc)] {
		case covHead:
			return // already decoded from here
		case covUnknown:
		default:
			d.conflicts = append(d.conflicts, fmt.Sprintf("$%04X: flow enters byte already claimed as %s", pc, covName(d.cov[d.idx(pc)])))
			return
		}
		in := z80dis.Decode(d.r.At, pc, nil)
		for i := 1; i < in.Len; i++ {
			a := pc + uint16(i)
			if !rom.Contains(a) || d.cov[d.idx(a)] != covUnknown {
				d.conflicts = append(d.conflicts, fmt.Sprintf("$%04X: %s overlaps byte $%04X already claimed", pc, in.Text, a))
				return
			}
		}
		d.cov[d.idx(pc)] = covHead
		for i := 1; i < in.Len; i++ {
			d.cov[d.idx(pc+uint16(i))] = covBody
		}
		d.insts[pc] = in
		if in.Invalid {
			d.conflicts = append(d.conflicts, fmt.Sprintf("$%04X: invalid opcode %s (misaligned decode?)", pc, in.Text))
		}
		if in.HasTarget {
			d.addTarget(in.Target, pc)
		}
		// LD rr,nn / PUSH rr pushes a continuation address that a later RET
		// will jump to: treat nn as code.
		if prevLoad.HasRef && rom.Contains(prevLoad.Ref) && pushesLoaded(prevLoad, in) {
			d.addTarget(prevLoad.Ref, pc)
		}
		prevLoad = in
		next := pc + uint16(in.Len)
		switch in.Flow {
		case z80dis.FlowJump, z80dis.FlowRet:
			return
		case z80dis.FlowIndirect:
			d.indirect = append(d.indirect, pc)
			return
		case z80dis.FlowCall:
			if inlineTableCallees[in.Target] {
				d.startInlineTable(next)
				return
			}
		}
		pc = next
	}
}

// pushesLoaded reports whether push is PUSH rr of the register pair that
// load (LD rr,nn) just set.
func pushesLoaded(load, push z80dis.Inst) bool {
	for _, rr := range []string{"BC", "DE", "HL", "IX", "IY"} {
		if strings.HasPrefix(load.Text, "LD "+rr+",$") && push.Text == "PUSH "+rr {
			return true
		}
	}
	return false
}

func (d *disasm) startInlineTable(a uint16) {
	if _, ok := d.tableAt[a]; ok {
		return
	}
	if n, ok := d.confirmed[a]; ok {
		d.layTable(a, n)
		return
	}
	d.labels[a] = true
	d.tableAt[a] = 0
	d.heuristic[a] = true
	d.growing = append(d.growing, &growTable{addr: a})
}

// layTable claims n table words at a and queues their targets.
func (d *disasm) layTable(a uint16, n int) {
	d.labels[a] = true
	d.tableAt[a] = n
	for i := 0; i < n; i++ {
		w := a + uint16(2*i)
		if !d.claimWord(w) {
			d.conflicts = append(d.conflicts, fmt.Sprintf("$%04X: confirmed table entry %d overlaps claimed bytes", w, i))
			continue
		}
		d.addTarget(d.r.Word(w), w)
	}
}

func (d *disasm) claimWord(w uint16) bool {
	if !rom.Contains(w) || !rom.Contains(w+1) ||
		d.cov[d.idx(w)] != covUnknown || d.cov[d.idx(w+1)] != covUnknown {
		return false
	}
	d.cov[d.idx(w)], d.cov[d.idx(w+1)] = covTable, covTable
	return true
}

// growOne extends one heuristic table by a single entry. It returns false
// once no table can grow.
func (d *disasm) growOne() bool {
	for _, g := range d.growing {
		if g.done {
			continue
		}
		w := g.addr + uint16(2*g.count)
		// Stop at another label: something else starts here.
		stop := g.count > 0 && d.labels[w]
		t := d.r.Word(w)
		if !stop && (!rom.Contains(t) || g.count >= 64) {
			stop = true
		}
		// A target inside an already-decoded instruction is a bad entry.
		if !stop && rom.Contains(t) {
			if c := d.cov[d.idx(t)]; c == covBody || c == covTable || c == covHeader {
				stop = true
			}
		}
		if stop || !d.claimWord(w) {
			g.done = true
			d.tableAt[g.addr] = g.count
			continue
		}
		g.count++
		d.tableAt[g.addr] = g.count
		d.addTarget(t, w)
		return true
	}
	return false
}

func (d *disasm) run() {
	for _, t := range tables {
		d.confirmed[t.addr] = t.count
	}
	for i := 0; i < 16; i++ {
		d.cov[i] = covHeader
	}
	for _, e := range entries {
		d.labels[e] = true
		d.queue = append(d.queue, e)
	}
	for _, t := range tables {
		d.layTable(t.addr, t.count)
	}
	for {
		for len(d.queue) > 0 {
			pc := d.queue[0]
			d.queue = d.queue[1:]
			d.walk(pc)
		}
		if !d.growOne() {
			return
		}
	}
}

func covName(c cover) string {
	return [...]string{"unknown", "code", "code", "table", "header"}[c]
}

func (d *disasm) write(w *bufio.Writer) {
	fmt.Fprintf(w, "; Recursive-descent disassembly of the cartridge at $%04X-$%04X.\n", rom.Base, rom.Base+rom.Size-1)
	fmt.Fprintf(w, "; Generated by cmd/disasm. Contains ROM bytes: never commit.\n")
	fmt.Fprintf(w, "; ROM SHA-1 verified: %v\n\n", d.r.Verified)

	a := uint16(rom.Base)
	end := rom.Base + rom.Size
	for int(a) < end {
		c := d.cov[d.idx(a)]
		if d.labels[a] || names[a] != "" && rom.Contains(a) {
			d.writeLabel(w, a)
		}
		switch c {
		case covHeader:
			d.writeHeader(w)
			a = rom.Base + 16
		case covHead:
			in := z80dis.DecodeWith(d.r.At, a, d.labeler, d.refLabeler)
			cmt := d.comment(in)
			line := fmt.Sprintf("    %04X  %-12s  %s", a, hexBytes(in.Bytes), in.Text)
			if cmt != "" {
				line = fmt.Sprintf("%-50s ; %s", line, cmt)
			}
			fmt.Fprintln(w, line)
			if in.Flow == z80dis.FlowJump || in.Flow == z80dis.FlowRet || in.Flow == z80dis.FlowIndirect ||
				(in.Flow == z80dis.FlowCall && inlineTableCallees[in.Target]) {
				fmt.Fprintln(w)
			}
			a += uint16(in.Len)
		case covTable:
			t := d.r.Word(a)
			note := ""
			if d.heuristic[d.tableStart(a)] {
				note = "  ; heuristic table entry: confirm"
			}
			fmt.Fprintf(w, "    %04X  %-12s  DW %s%s\n", a, hexBytes([]byte{d.r.At(a), d.r.At(a + 1)}), d.labeler(t), note)
			a += 2
		default:
			// Unknown bytes: dump 8 per line, breaking at labels and claimed bytes.
			start := a
			var bs []byte
			for int(a) < end && d.cov[d.idx(a)] == covUnknown && len(bs) < 8 && (a == start || !d.labels[a]) {
				bs = append(bs, d.r.At(a))
				a++
			}
			fmt.Fprintf(w, "    %04X  %-12s  DB %s  ; %s\n", start, "", dbList(bs), ascii(bs))
		}
	}
}

func (d *disasm) tableStart(a uint16) uint16 {
	for a > rom.Base {
		if _, ok := d.tableAt[a]; ok {
			return a
		}
		a -= 2
	}
	return a
}

func (d *disasm) writeLabel(w *bufio.Writer, a uint16) {
	refs := d.xrefs[a]
	if len(refs) == 0 {
		fmt.Fprintf(w, "%s:\n", d.label(a))
		return
	}
	var rs []string
	for i, r := range refs {
		if i == 6 {
			rs = append(rs, fmt.Sprintf("+%d more", len(refs)-6))
			break
		}
		rs = append(rs, fmt.Sprintf("$%04X", r))
	}
	fmt.Fprintf(w, "%-40s ; xref %s\n", d.label(a)+":", strings.Join(rs, " "))
}

func (d *disasm) writeHeader(w *bufio.Writer) {
	fmt.Fprintf(w, "    4000  %-12s  DB \"AB\"            ; cartridge ID\n", hexBytes([]byte{d.r.At(0x4000), d.r.At(0x4001)}))
	for i, n := range []string{"INIT", "STATEMENT", "DEVICE", "TEXT"} {
		a := uint16(0x4002 + 2*i)
		fmt.Fprintf(w, "    %04X  %-12s  DW %-14s ; %s\n", a, hexBytes([]byte{d.r.At(a), d.r.At(a + 1)}), d.refLabeler(d.r.Word(a)), n)
	}
	fmt.Fprintf(w, "    400A  %-12s  DB %s  ; reserved\n\n", "", dbList(d.r.Data[10:16]))
}

func (d *disasm) comment(in z80dis.Inst) string {
	if in.HasRef {
		if n, ok := names[in.Ref]; ok && in.Ref >= 0xF000 {
			return n
		}
		if in.Ref >= 0xE000 && in.Ref < 0xF000 {
			return fmt.Sprintf("RAM[$%03X]", in.Ref-0xE000)
		}
	}
	return ""
}

func hexBytes(b []byte) string {
	s := make([]string, len(b))
	for i, x := range b {
		s[i] = fmt.Sprintf("%02X", x)
	}
	return strings.Join(s, " ")
}

func dbList(b []byte) string {
	s := make([]string, len(b))
	for i, x := range b {
		s[i] = fmt.Sprintf("$%02X", x)
	}
	return strings.Join(s, ",")
}

func ascii(b []byte) string {
	var sb strings.Builder
	for _, x := range b {
		if x >= 0x20 && x < 0x7F {
			sb.WriteByte(x)
		} else {
			sb.WriteByte('.')
		}
	}
	return sb.String()
}

func main() {
	romPath := flag.String("rom", rom.DefaultPath, "cartridge image")
	out := flag.String("out", "docs/listing.asm", "listing output path")
	gaps := flag.Bool("gaps", false, "list unclaimed byte ranges")
	peek := flag.String("peek", "", "linear-decode a range START-END (hex) to stdout without claiming it, then exit")
	flag.Parse()

	r, err := rom.Load(*romPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r.WarnIfUnverified()

	if *peek != "" {
		var lo, hi uint16
		if _, err := fmt.Sscanf(strings.ReplaceAll(*peek, "$", ""), "%x-%x", &lo, &hi); err != nil {
			fmt.Fprintln(os.Stderr, "-peek wants START-END in hex, e.g. 45E7-4620")
			os.Exit(2)
		}
		for a := lo; a <= hi && a >= lo; {
			in := z80dis.Decode(r.At, a, nil)
			fmt.Printf("    %04X  %-12s  %s\n", a, hexBytes(in.Bytes), in.Text)
			a += uint16(in.Len)
		}
		return
	}

	d := &disasm{
		r:         r,
		insts:     map[uint16]z80dis.Inst{},
		labels:    map[uint16]bool{},
		xrefs:     map[uint16][]uint16{},
		tableAt:   map[uint16]int{},
		heuristic: map[uint16]bool{},
		confirmed: map[uint16]int{},
	}
	d.run()

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bw := bufio.NewWriter(f)
	d.write(bw)
	bw.Flush()
	f.Close()

	d.report(*out)
	if *gaps {
		d.reportGaps()
	}
}

func sortedKeys(m map[uint16]z80dis.Inst) []uint16 {
	ks := make([]uint16, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return ks[i] < ks[j] })
	return ks
}

// reportGaps lists each run of unclaimed bytes with the labels or xrefs that
// point into it, to help separate data from unreached code.
func (d *disasm) reportGaps() {
	fmt.Println("unclaimed ranges:")
	for i := 0; i < rom.Size; {
		if d.cov[i] != covUnknown {
			i++
			continue
		}
		j := i
		for j < rom.Size && d.cov[j] == covUnknown {
			j++
		}
		fmt.Printf("  $%04X-$%04X %5d bytes\n", rom.Base+i, rom.Base+j-1, j-i)
		lo, hi := uint16(rom.Base+i), uint16(rom.Base+j-1)
		var refs []string
		for _, a := range sortedKeys(d.insts) {
			in := d.insts[a]
			if in.HasRef && in.Ref >= lo && in.Ref <= hi {
				refs = append(refs, fmt.Sprintf("$%04X->$%04X (%s)", a, in.Ref, in.Text))
			}
		}
		for _, r := range refs {
			fmt.Println("      ref", r)
		}
		i = j
	}
}

func (d *disasm) report(out string) {
	var n [5]int
	for _, c := range d.cov {
		n[c]++
	}
	code := n[covHead] + n[covBody]
	fmt.Printf("wrote %s\n", out)
	fmt.Printf("coverage: code %d (%.1f%%), tables %d, header %d, unknown %d (%.1f%%); %d instructions\n",
		code, 100*float64(code)/rom.Size, n[covTable], n[covHeader], n[covUnknown],
		100*float64(n[covUnknown])/rom.Size, len(d.insts))

	var ts []uint16
	for a := range d.tableAt {
		ts = append(ts, a)
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	for _, a := range ts {
		kind := "confirmed"
		if d.heuristic[a] {
			kind = "HEURISTIC (confirm and add to hints.go)"
		}
		fmt.Printf("table $%04X: %d entries, %s\n", a, d.tableAt[a], kind)
	}

	sort.Slice(d.indirect, func(i, j int) bool { return d.indirect[i] < d.indirect[j] })
	var ind []string
	for _, a := range d.indirect {
		ind = append(ind, fmt.Sprintf("$%04X", a))
	}
	fmt.Printf("indirect jumps reached (%d): %s\n", len(ind), strings.Join(ind, " "))
	for _, c := range d.conflicts {
		fmt.Println("CONFLICT", c)
	}
}
