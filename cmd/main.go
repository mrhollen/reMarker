package main

import (
	"log"

	"github.com/hollen/remarker/internal/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
