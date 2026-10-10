// Command extract rebuilds every asset in internal/manifest from a
// user-supplied ROM and writes them to assets/*.bin (gitignored).
//
//	go run ./cmd/extract          write assets/
//	go run ./cmd/extract -check   re-derive and diff against assets/ without writing
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go-msx-treegame/internal/manifest"
	"go-msx-treegame/internal/rom"
)

func main() {
	romPath := flag.String("rom", rom.DefaultPath, "cartridge image")
	dir := flag.String("dir", "assets", "asset directory")
	check := flag.Bool("check", false, "verify assets/ matches the ROM without writing")
	flag.Parse()

	r, err := rom.Load(*romPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r.WarnIfUnverified()

	files, err := manifest.Extract(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(1)
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)

	bad := 0
	for _, n := range names {
		path := filepath.Join(*dir, n+".bin")
		if *check {
			have, err := os.ReadFile(path)
			switch {
			case err != nil:
				fmt.Printf("MISSING  %s\n", path)
				bad++
			case !bytes.Equal(have, files[n]):
				fmt.Printf("DIFFERS  %s\n", path)
				bad++
			}
			continue
		}
		if err := os.WriteFile(path, files[n], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	// Stale files: on disk but no longer produced by the manifest.
	existing, _ := filepath.Glob(filepath.Join(*dir, "*.bin"))
	for _, p := range existing {
		n := filepath.Base(p)
		if _, ok := files[n[:len(n)-4]]; !ok {
			fmt.Printf("STALE    %s (not in manifest)\n", p)
			bad++
		}
	}
	if *check {
		if bad > 0 {
			fmt.Printf("check: %d problem(s) in %d assets\n", bad, len(names))
			os.Exit(1)
		}
		fmt.Printf("check: all %d assets match the ROM\n", len(names))
		return
	}
	fmt.Printf("wrote %d assets to %s/\n", len(names), *dir)
}
