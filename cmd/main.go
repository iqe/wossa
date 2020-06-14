package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	log "github.com/inconshreveable/log15"

	wossamessa "github.com/iqe/wossamessa/internal"
)

var (
	version = "undefined" // updated during release build
)

func main() {
	device := flag.String("d", "/dev/video0", "Video device to use")
	configDir := flag.String("c", ".", "Directory for config and data files")
	apiAddr := flag.String("l", "0.0.0.0:8080", "Host:port for HTTP API")
	verbose := flag.Bool("v", false, "Print more verbose messages")
	versionFlag := flag.Bool("V", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("wossamessa - version %s\n", version)
		os.Exit(0)
	}

	logLevel := log.LvlInfo
	if *verbose {
		logLevel = log.LvlDebug
	}
	log.Root().SetHandler(log.LvlFilterHandler(logLevel, log.StdoutHandler))

	wossamessa.ConfigDir = *configDir
	go func() {
		err := wossamessa.RunWebCam(*device)
		if err != nil {
			log.Error("Error while running webcam", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		err := wossamessa.RunApi(*apiAddr, *verbose)
		if err != nil {
			log.Error("Error while running API", "error", err)
			os.Exit(1)
		}
	}()

	waitForCtrlC()

	// Write the current meter to disk on clean exit
	err := wossamessa.PersistMeter()
	if err != nil {
		log.Error("Error while persisting meter", "error", err)
	}
}

func waitForCtrlC() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	<-signals
}
