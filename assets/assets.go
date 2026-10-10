// Package assets embeds the files cmd/extract derives from the user's ROM.
// The .bin files are gitignored: run `go run ./cmd/extract` before building.
package assets

import (
	"embed"
	"fmt"
)

//go:embed *.bin
var files embed.FS

// Get returns the named asset (without the .bin suffix). It panics if the
// asset is missing, which means cmd/extract has not been run or the
// manifest and the engine disagree.
func Get(name string) []byte {
	b, err := files.ReadFile(name + ".bin")
	if err != nil {
		panic(fmt.Sprintf("assets: %s missing; run go run ./cmd/extract", name))
	}
	return b
}
