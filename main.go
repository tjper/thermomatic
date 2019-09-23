package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tjper/thermomatic/internal/server"
)

// addr is the default TCP listening port
const addr = 1337

// httpAddr is the default HTTP listening port
const httpAddr = 1338

func main() {
	svr, err := server.New(
		addr,
		server.WithHttpServer(httpAddr),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer svr.Shutdown()

	go svr.ListenAndServe()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)
}
