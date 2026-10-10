package game

import "testing"

// Starting from the menu with the cheat reaches play at the chosen stage.
func TestCheatStart(t *testing.T) {
	for _, n := range []int{1, 5, 9} {
		c := New()
		c.Boot()
		for i := 0; i < 2000 && c.RAM[0x000] != 5; i++ {
			c.Frame()
		}
		if c.RAM[0x000] != 5 {
			t.Fatal("menu not reached")
		}
		c.CheatStart(n)
		for i := 0; i < 2000 && c.RAM[0x000] != 11; i++ {
			c.Frame()
		}
		if c.RAM[0x000] != 11 {
			t.Fatalf("stage %d: play not reached, state %02X", n, c.RAM[0x000])
		}
		if c.RAM[0x05C] != byte(n-1) || c.RAM[0x051] != byte(n) {
			t.Errorf("stage %d: index %d, number %02X", n, c.RAM[0x05C], c.RAM[0x051])
		}
		for i := 0; i < 600; i++ { // play a while with no input
			c.Frame()
		}
	}
}

// The cheat does nothing outside the menu.
func TestCheatOnlyOnMenu(t *testing.T) {
	c := New()
	c.Boot()
	c.Frame()
	c.CheatStart(3)
	if c.startStage != 0 || c.RAM[0x000] == 8 {
		t.Error("cheat acted outside the menu")
	}
}

// Dying above the bottom of a cheat-started stage falls back to the bottom
// of the map and restarts, instead of scrolling past the end of the map.
func TestCheatDeathAfterClimbing(t *testing.T) {
	for _, n := range []int{1, 2, 5, 9} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("stage %d: panic: %v", n, r)
				}
			}()
			c := New()
			c.Boot()
			for c.RAM[0x000] != 5 {
				c.Frame()
			}
			c.CheatStart(n)
			for c.RAM[0x000] != 11 {
				c.Frame()
			}
			for i := 0; i < 12; i++ { // as if the player had climbed
				c.scrollUp()
			}
			c.RAM[0x1B2], c.RAM[0x003], c.RAM[0x001] = 8, 0, 1 // die ($525D)
			lives := c.RAM[0x050]
			for i := 0; i < 3000 && !(c.RAM[0x000] == 11 && c.RAM[0x050] < lives); i++ {
				c.Frame()
			}
			if c.RAM[0x000] != 11 || c.RAM[0x050] != lives-1 {
				t.Errorf("stage %d: no restart after death: state %02X lives %d", n, c.RAM[0x000], c.RAM[0x050])
			}
		}()
	}
}

// The derived bottom type agrees with new-game data for stage 1 ($84) and
// with the map the game actually builds for every stage.
func TestBottomRecordType(t *testing.T) {
	if got := bottomRecordType(0); got != 0x84 {
		t.Errorf("stage 1 bottom type %02X, want $84 (new-game RAM[$056])", got)
	}
	for idx := byte(0); idx < 9; idx++ {
		c := New()
		c.RAM[0x05C] = idx
		c.buildStageMap()
		if got, want := bottomRecordType(idx), c.VRAM[0x3B81]; got != want {
			t.Errorf("stage index %d: derived %02X, built map has %02X", idx, got, want)
		}
	}
}
