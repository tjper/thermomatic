package main

import (
	"log"

	"github.com/tjper/thermomatic/internal/server"
)

func main() {
	if err := server.Start(":1337"); err != nil {
		log.Fatal(err)
	}
}
