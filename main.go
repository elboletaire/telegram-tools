package main

import (
	"log"

	"github.com/elboletaire/ttools/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
