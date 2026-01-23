package main

import (
	"log"
	"os"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server"
)

func main() {
	log.SetPrefix("scribble: ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix)

	log.Println("loading configuration...")
	configFile := os.Getenv("CONFIG_FILE")
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
		return
	}

	log.Println("starting server...")
	if err := server.StartServer(cfg); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
