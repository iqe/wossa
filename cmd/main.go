package main

import (
	"flag"

	wossamessa "github.com/iqe/wossamessa/internal"
)

func main() {
	device := flag.String("d", "/dev/video0", "Video device to use")
	configDir := flag.String("c", ".", "Directory for config and data files")
	flag.Parse()

	wossamessa.ConfigDir = *configDir
	go wossamessa.RunWebCam(*device)
	wossamessa.Run()
}
