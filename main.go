package main

import (
	_ "embed"
	"log"

	"csnative/internal/desktop"
)

//go:embed build/appicon.png
var appIcon []byte

func main() {
	if err := desktop.Run(appIcon); err != nil {
		log.Fatal(err)
	}
}
