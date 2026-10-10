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
