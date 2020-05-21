package main

import (
	"flag"

	wossamessa "github.com/iqe/wossamessa/internal"
)

func main() {
	device := flag.String("d", "/dev/video0", "video device to use")
	flag.Parse()

	go wossamessa.RunWebCam(*device)
	wossamessa.Run()
}
