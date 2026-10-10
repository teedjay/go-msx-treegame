// Command play runs the port in a window.
//
//	go run ./cmd/play [-scale 3] [-record keys.txt]
//
// Controls: arrow keys move, SPACE is trigger A (jump / start), and Left
// Shift is SELECT (trigger B). Both the keyboard and joystick port 1 see
// them, so every menu choice works. Esc quits. -mute disables sound.
// Cheat: on the menu screen, keys 1-9 start the game at that stage.
//
// -record writes the keyboard changes per tick in the REFDUMP_KEYS format
// of scripts/refdump.tcl, so a session played with a KEYBOARD menu choice
// can be replayed into the original and checked with cmd/refcompare.
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-msx-treegame/internal/audio"
	"go-msx-treegame/internal/game"
	"go-msx-treegame/internal/msxrender"
)

type player struct {
	c      *game.Console
	frame  *image.RGBA
	screen *ebiten.Image
	tick   int
	rec    *os.File
	prev   [2]byte // last recorded rows 7 and 8, active-high
	sound  *audio.Stream
}

func (p *player) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	row7, row8, joy := readKeys()
	in := &p.c.Input
	in.ReleaseAll()
	in.KeyRow[7] = ^row7
	in.KeyRow[8] = ^row8
	in.Joy[0] = ^joy
	if p.rec != nil {
		p.record(row7, row8)
	}
	// Cheat: on the menu, 1-9 starts at that stage in the selected mode.
	for n := 1; n <= 9; n++ {
		if inpututil.IsKeyJustPressed(ebiten.Key1 + ebiten.Key(n-1)) {
			p.c.CheatStart(n)
		}
	}
	p.c.Frame()
	if p.sound != nil {
		p.sound.PSG.SetRegs(p.c.PSG)
	}
	p.tick++
	return nil
}

// readKeys returns keyboard rows 7 and 8 and the joystick byte, active-high.
func readKeys() (row7, row8, joy byte) {
	k := ebiten.IsKeyPressed
	if k(ebiten.KeyShiftLeft) {
		row7 |= 0x40 // SELECT
		joy |= game.BtnB
	}
	if k(ebiten.KeySpace) {
		row8 |= 0x01
		joy |= game.BtnA
	}
	if k(ebiten.KeyArrowLeft) {
		row8 |= 0x10
		joy |= game.BtnLeft
	}
	if k(ebiten.KeyArrowUp) {
		row8 |= 0x20
		joy |= game.BtnUp
	}
	if k(ebiten.KeyArrowDown) {
		row8 |= 0x40
		joy |= game.BtnDown
	}
	if k(ebiten.KeyArrowRight) {
		row8 |= 0x80
		joy |= game.BtnRight
	}
	return
}

// record logs key changes for this tick as "tick:row:mask:down|up".
func (p *player) record(row7, row8 byte) {
	var out []string
	for i, now := range [2]byte{row7, row8} {
		row := 7 + i
		was := p.prev[i]
		if down := now &^ was; down != 0 {
			out = append(out, fmt.Sprintf("%d:%d:%d:down", p.tick, row, down))
		}
		if up := was &^ now; up != 0 {
			out = append(out, fmt.Sprintf("%d:%d:%d:up", p.tick, row, up))
		}
		p.prev[i] = now
	}
	if len(out) > 0 {
		fmt.Fprintln(p.rec, strings.Join(out, " "))
	}
}

func (p *player) Draw(screen *ebiten.Image) {
	msxrender.RenderInto(p.frame, &p.c.VRAM, &p.c.Reg)
	p.screen.WritePixels(p.frame.Pix)
	screen.DrawImage(p.screen, nil)
}

func (p *player) Layout(int, int) (int, int) { return msxrender.Width, msxrender.Height }

func main() {
	scale := flag.Int("scale", 3, "window scale")
	recPath := flag.String("record", "", "write per-tick keyboard changes to this file")
	mute := flag.Bool("mute", false, "no sound")
	flag.Parse()

	p := &player{
		c:      game.New(),
		frame:  image.NewRGBA(image.Rect(0, 0, msxrender.Width, msxrender.Height)),
		screen: ebiten.NewImage(msxrender.Width, msxrender.Height),
	}
	if *recPath != "" {
		f, err := os.Create(*recPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		p.rec = f
	}
	p.c.Boot()
	if !*mute {
		const rate = 44100
		p.sound = audio.NewStream(rate)
		ap, err := ebaudio.NewContext(rate).NewPlayer(p.sound)
		if err != nil {
			log.Fatal(err)
		}
		ap.SetBufferSize(60 * time.Millisecond) // low latency
		ap.Play()
	}

	ebiten.SetWindowTitle("go-msx-treegame")
	ebiten.SetWindowSize(msxrender.Width**scale, msxrender.Height**scale)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(p); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
