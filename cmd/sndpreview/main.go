// Command sndpreview renders a sound through the ported sequencer and the
// PSG model to a WAV file, frame by frame at 60 Hz.
//
//	go run ./cmd/sndpreview -id 0x8C -out /tmp/tune.wav
//
// The output is derived from ROM data: write it outside the repository.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"

	"go-msx-treegame/internal/audio"
	"go-msx-treegame/internal/game"
)

const rate = 44100

func main() {
	id := flag.Uint("id", 0x8C, "sound id")
	out := flag.String("out", "/tmp/sound.wav", "output WAV")
	maxSec := flag.Int("max", 60, "maximum seconds")
	flag.Parse()

	c := game.New()
	c.Boot()
	// Let the boot-time sound $9F finish.
	for i := 0; i < 600 && c.DebugSoundBusy(); i++ {
		c.SoundTick()
	}
	c.DebugPlaySound(byte(*id))
	s := audio.NewStream(rate)
	var pcm []byte
	frame := make([]byte, rate/60*4)
	frames := 0
	for ; frames < *maxSec*60; frames++ {
		c.SoundTick()
		s.PSG.SetRegs(c.PSG)
		s.Read(frame)
		pcm = append(pcm, frame...)
		if !c.DebugSoundBusy() {
			break
		}
	}
	// 16-bit stereo -> mono WAV.
	mono := make([]byte, len(pcm)/2)
	for i := 0; i < len(pcm)/4; i++ {
		copy(mono[i*2:], pcm[i*4:i*4+2])
	}
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	hdr := make([]byte, 44)
	copy(hdr, "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(mono)))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1)
	binary.LittleEndian.PutUint16(hdr[22:], 1)
	binary.LittleEndian.PutUint32(hdr[24:], rate)
	binary.LittleEndian.PutUint32(hdr[28:], rate*2)
	binary.LittleEndian.PutUint16(hdr[32:], 2)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(mono)))
	f.Write(hdr)
	f.Write(mono)
	fmt.Printf("%s: sound $%02X, %d frames (%.1f s)\n", *out, *id, frames, float64(frames)/60)
}
