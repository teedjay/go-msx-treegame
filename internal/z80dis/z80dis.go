// Package z80dis decodes single Z80 instructions, including the CB, ED, DD,
// FD and DDCB/FDCB prefixed sets and the undocumented forms (IXH/IXL, SLL,
// DDCB register copies). It reports control flow so a caller can do
// recursive-descent disassembly.
//
// Decoding follows the x/y/z/p/q opcode decomposition:
//
//	x = op>>6, y = (op>>3)&7, z = op&7, p = y>>1, q = y&1
package z80dis

import "fmt"

// Flow describes how an instruction affects control flow.
type Flow int

const (
	FlowNext     Flow = iota // falls through to the next instruction
	FlowJump                 // unconditional jump to Target (JP nn, JR e)
	FlowBranch               // conditional jump to Target, else falls through
	FlowCall                 // call to Target, then falls through on return
	FlowRet                  // unconditional return; does not fall through
	FlowCondRet              // conditional return; falls through
	FlowIndirect             // JP (HL)/(IX)/(IY): target unknown statically
)

// Inst is one decoded instruction.
type Inst struct {
	Addr   uint16
	Len    int
	Bytes  []byte
	Text   string // mnemonic and operands
	Flow   Flow
	Target uint16 // valid when HasTarget
	// HasTarget is set for jumps, branches, calls and RST.
	HasTarget bool
	// Ref is a 16-bit immediate or absolute memory operand that may be an
	// address (LD HL,nn; LD A,(nn); ...). Valid when HasRef.
	Ref    uint16
	HasRef bool
	// Invalid marks an opcode with no defined meaning (unknown ED xx).
	Invalid bool
}

// Fetcher returns the byte at a 16-bit address.
type Fetcher func(addr uint16) byte

// Labeler formats an address operand. It may return a label name.
type Labeler func(addr uint16) string

// DefaultLabeler formats addresses as $XXXX.
func DefaultLabeler(a uint16) string { return fmt.Sprintf("$%04X", a) }

var (
	regs   = [8]string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}
	rpTab  = [4]string{"BC", "DE", "HL", "SP"}
	rp2Tab = [4]string{"BC", "DE", "HL", "AF"}
	ccTab  = [8]string{"NZ", "Z", "NC", "C", "PO", "PE", "P", "M"}
	aluTab = [8]string{"ADD A,", "ADC A,", "SUB ", "SBC A,", "AND ", "XOR ", "OR ", "CP "}
	rotTab = [8]string{"RLC", "RRC", "RL", "RR", "SLA", "SRA", "SLL", "SRL"}
	imTab  = [8]string{"0", "0/1", "1", "2", "0", "0/1", "1", "2"}
	acc    = [8]string{"RLCA", "RRCA", "RLA", "RRA", "DAA", "CPL", "SCF", "CCF"}
	bli    = [4][4]string{
		{"LDI", "CPI", "INI", "OUTI"},
		{"LDD", "CPD", "IND", "OUTD"},
		{"LDIR", "CPIR", "INIR", "OTIR"},
		{"LDDR", "CPDR", "INDR", "OTDR"},
	}
)

type decoder struct {
	f      Fetcher
	lab    Labeler
	refLab Labeler
	pc  uint16 // start address
	n   int    // bytes consumed
	in  Inst
	// index-register mode: "" (HL), "IX" or "IY"
	ix   string
	disp int8
	// dispRead is set once the (IX+d) displacement has been consumed.
	dispRead bool
}

func (d *decoder) next() byte {
	b := d.f(d.pc + uint16(d.n))
	d.n++
	return b
}

func (d *decoder) imm16() uint16 {
	lo := uint16(d.next())
	return lo | uint16(d.next())<<8
}

func (d *decoder) addr(a uint16) string {
	d.in.Ref, d.in.HasRef = a, true
	return d.refLab(a)
}

func (d *decoder) target(a uint16, fl Flow) string {
	d.in.Target, d.in.HasTarget, d.in.Flow = a, true, fl
	return d.lab(a)
}

func (d *decoder) rel() uint16 {
	e := int8(d.next())
	return uint16(int(d.pc) + d.n + int(e))
}

// hl returns HL, IX or IY.
func (d *decoder) hl() string {
	if d.ix != "" {
		return d.ix
	}
	return "HL"
}

// mem returns (HL) or (IX+d), reading the displacement on first use.
func (d *decoder) mem() string {
	if d.ix == "" {
		return "(HL)"
	}
	if !d.dispRead {
		d.disp = int8(d.next())
		d.dispRead = true
	}
	if d.disp < 0 {
		return fmt.Sprintf("(%s-$%02X)", d.ix, -int(d.disp))
	}
	return fmt.Sprintf("(%s+$%02X)", d.ix, int(d.disp))
}

// reg returns register r[i]. useIdxHL selects IXH/IXL for H/L; it must be
// false when the same instruction also addresses (IX+d).
func (d *decoder) reg(i int, useIdxHL bool) string {
	switch {
	case i == 6:
		return d.mem()
	case d.ix != "" && useIdxHL && i == 4:
		return d.ix + "H"
	case d.ix != "" && useIdxHL && i == 5:
		return d.ix + "L"
	}
	return regs[i]
}

func (d *decoder) rp(p int) string {
	if p == 2 {
		return d.hl()
	}
	return rpTab[p]
}

func (d *decoder) rp2(p int) string {
	if p == 2 {
		return d.hl()
	}
	return rp2Tab[p]
}

// Decode decodes the instruction at pc. lab formats jump/call targets and
// immediate or memory operands alike; it may be nil.
func Decode(f Fetcher, pc uint16, lab Labeler) Inst {
	return DecodeWith(f, pc, lab, lab)
}

// DecodeWith is Decode with separate formatters for control-flow targets
// (lab) and for immediate or absolute memory operands (refLab), so that a
// constant like LD DE,$0000 is not printed as a BIOS routine name.
func DecodeWith(f Fetcher, pc uint16, lab, refLab Labeler) Inst {
	if lab == nil {
		lab = DefaultLabeler
	}
	if refLab == nil {
		refLab = DefaultLabeler
	}
	d := &decoder{f: f, lab: lab, refLab: refLab, pc: pc}
	d.in.Addr = pc
	d.in.Text = d.decode()
	d.in.Len = d.n
	d.in.Bytes = make([]byte, d.n)
	for i := range d.in.Bytes {
		d.in.Bytes[i] = f(pc + uint16(i))
	}
	return d.in
}

func (d *decoder) decode() string {
	op := d.next()
	switch op {
	case 0xCB:
		return d.decodeCB()
	case 0xED:
		return d.decodeED()
	case 0xDD, 0xFD:
		nx := d.f(d.pc + uint16(d.n))
		if nx == 0xDD || nx == 0xFD || nx == 0xED {
			// A prefix followed by another prefix acts as a NOP.
			d.in.Invalid = true
			return "NOP*"
		}
		if op == 0xDD {
			d.ix = "IX"
		} else {
			d.ix = "IY"
		}
		op = d.next()
		if op == 0xCB {
			return d.decodeIdxCB()
		}
	}
	return d.decodeMain(op)
}

func (d *decoder) decodeMain(op byte) string {
	x, y, z := int(op>>6), int(op>>3)&7, int(op&7)
	p, q := y>>1, y&1
	switch x {
	case 0:
		switch z {
		case 0:
			switch y {
			case 0:
				return "NOP"
			case 1:
				return "EX AF,AF'"
			case 2:
				t := d.rel()
				return "DJNZ " + d.target(t, FlowBranch)
			case 3:
				t := d.rel()
				return "JR " + d.target(t, FlowJump)
			default:
				t := d.rel()
				return "JR " + ccTab[y-4] + "," + d.target(t, FlowBranch)
			}
		case 1:
			if q == 0 {
				nn := d.imm16()
				return "LD " + d.rp(p) + "," + d.addr(nn)
			}
			return "ADD " + d.hl() + "," + d.rp(p)
		case 2:
			switch y {
			case 0:
				return "LD (BC),A"
			case 1:
				return "LD A,(BC)"
			case 2:
				return "LD (DE),A"
			case 3:
				return "LD A,(DE)"
			case 4:
				return "LD (" + d.addr(d.imm16()) + ")," + d.hl()
			case 5:
				return "LD " + d.hl() + ",(" + d.addr(d.imm16()) + ")"
			case 6:
				return "LD (" + d.addr(d.imm16()) + "),A"
			default:
				return "LD A,(" + d.addr(d.imm16()) + ")"
			}
		case 3:
			if q == 0 {
				return "INC " + d.rp(p)
			}
			return "DEC " + d.rp(p)
		case 4:
			return "INC " + d.reg(y, true)
		case 5:
			return "DEC " + d.reg(y, true)
		case 6:
			r := d.reg(y, true) // reads displacement before n
			return fmt.Sprintf("LD %s,$%02X", r, d.next())
		default:
			return acc[y]
		}
	case 1:
		if z == 6 && y == 6 {
			return "HALT"
		}
		usesMem := y == 6 || z == 6
		dst := d.reg(y, !usesMem)
		src := d.reg(z, !usesMem)
		return "LD " + dst + "," + src
	case 2:
		return aluTab[y] + d.reg(z, true)
	}
	// x == 3
	switch z {
	case 0:
		d.in.Flow = FlowCondRet
		return "RET " + ccTab[y]
	case 1:
		if q == 0 {
			return "POP " + d.rp2(p)
		}
		switch p {
		case 0:
			d.in.Flow = FlowRet
			return "RET"
		case 1:
			return "EXX"
		case 2:
			d.in.Flow = FlowIndirect
			return "JP (" + d.hl() + ")"
		default:
			return "LD SP," + d.hl()
		}
	case 2:
		t := d.imm16()
		return "JP " + ccTab[y] + "," + d.target(t, FlowBranch)
	case 3:
		switch y {
		case 0:
			t := d.imm16()
			return "JP " + d.target(t, FlowJump)
		case 2:
			return fmt.Sprintf("OUT ($%02X),A", d.next())
		case 3:
			return fmt.Sprintf("IN A,($%02X)", d.next())
		case 4:
			return "EX (SP)," + d.hl()
		case 5:
			return "EX DE,HL"
		case 6:
			return "DI"
		case 7:
			return "EI"
		}
		// y == 1 (CB) is handled before decodeMain.
	case 4:
		t := d.imm16()
		return "CALL " + ccTab[y] + "," + d.target(t, FlowCall)
	case 5:
		if q == 0 {
			return "PUSH " + d.rp2(p)
		}
		// p == 0 is CALL nn; DD/ED/FD are handled before decodeMain.
		t := d.imm16()
		return "CALL " + d.target(t, FlowCall)
	case 6:
		return fmt.Sprintf("%s$%02X", aluTab[y], d.next())
	}
	// z == 7
	t := uint16(y * 8)
	return "RST " + d.target(t, FlowCall)
}

func (d *decoder) decodeCB() string {
	op := d.next()
	x, y, z := int(op>>6), int(op>>3)&7, int(op&7)
	r := regs[z]
	switch x {
	case 0:
		return rotTab[y] + " " + r
	case 1:
		return fmt.Sprintf("BIT %d,%s", y, r)
	case 2:
		return fmt.Sprintf("RES %d,%s", y, r)
	}
	return fmt.Sprintf("SET %d,%s", y, r)
}

// decodeIdxCB handles DD CB d op / FD CB d op.
func (d *decoder) decodeIdxCB() string {
	m := d.mem() // displacement comes before the opcode
	op := d.next()
	x, y, z := int(op>>6), int(op>>3)&7, int(op&7)
	var s string
	switch x {
	case 0:
		s = rotTab[y] + " " + m
	case 1:
		return fmt.Sprintf("BIT %d,%s", y, m) // no register copy for BIT
	case 2:
		s = fmt.Sprintf("RES %d,%s", y, m)
	default:
		s = fmt.Sprintf("SET %d,%s", y, m)
	}
	if z != 6 {
		s += "," + regs[z] // undocumented: result also copied to register
	}
	return s
}

func (d *decoder) decodeED() string {
	op := d.next()
	x, y, z := int(op>>6), int(op>>3)&7, int(op&7)
	p, q := y>>1, y&1
	if x == 1 {
		switch z {
		case 0:
			if y == 6 {
				return "IN (C)"
			}
			return "IN " + regs[y] + ",(C)"
		case 1:
			if y == 6 {
				return "OUT (C),0"
			}
			return "OUT (C)," + regs[y]
		case 2:
			if q == 0 {
				return "SBC HL," + rpTab[p]
			}
			return "ADC HL," + rpTab[p]
		case 3:
			nn := d.imm16()
			if q == 0 {
				return "LD (" + d.addr(nn) + ")," + rpTab[p]
			}
			return "LD " + rpTab[p] + ",(" + d.addr(nn) + ")"
		case 4:
			return "NEG"
		case 5:
			d.in.Flow = FlowRet
			if y == 1 {
				return "RETI"
			}
			return "RETN"
		case 6:
			return "IM " + imTab[y]
		default:
			return [8]string{"LD I,A", "LD R,A", "LD A,I", "LD A,R", "RRD", "RLD", "NOP*", "NOP*"}[y]
		}
	}
	if x == 2 && z <= 3 && y >= 4 {
		return bli[y-4][z]
	}
	d.in.Invalid = true
	return fmt.Sprintf("DB $ED,$%02X", op)
}
