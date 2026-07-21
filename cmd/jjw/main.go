package main

import (
	"os"

	"github.com/Cyberistic/jjCoW/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
