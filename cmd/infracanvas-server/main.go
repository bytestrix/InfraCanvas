package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"infracanvas/pkg/server"
)

var version = "0.1.0"

func main() {
	addr := flag.String("addr", ":8080", "Listen address (host:port)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("infracanvas-server %s\n", version)
		os.Exit(0)
	}

	// Allow overriding via environment variable
	if envAddr := os.Getenv("INFRACANVAS_ADDR"); envAddr != "" {
		*addr = envAddr
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("infracanvas-server %s", version)

	srv := server.New()
	if err := srv.ListenAndServe(*addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
