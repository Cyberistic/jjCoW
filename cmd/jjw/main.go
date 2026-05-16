package main

import (
	"os"

	"github.com/aranw/jjw/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
