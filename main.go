package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tjper/thermomatic/internal/server"
)

// addr is the default listening port
const addr = 1337

func main() {
	svr, err := server.New(addr)
	if err != nil {
		log.Fatal(err)
	}
	defer svr.Shutdown()

	go svr.Accept()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)
}
