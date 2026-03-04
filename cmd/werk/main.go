package main

import (
	"os"

	"werk/internal/commands"
	"werk/web"
)

func main() {
	commands.WebFS = web.FS
	if err := commands.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
