package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server"
)

func main() {
	log.SetPrefix("scribble: ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix)

	configFile := flag.String("config", "config.yml", "Path to the configuration file (i.e., /etc/scribble.yaml)")
	flag.Parse()

	if len(strings.Trim(*configFile, " ")) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	log.Println("loading configuration...")
	config.LoadAndValidateConfiguration(*configFile)

	log.Println("starting http server...")
	server.StartServer()
}
