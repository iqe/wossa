package main

import (
	wossamessa "github.com/iqe/wossamessa/internal"
)

func main() {
	//device := flag.String("d", "/dev/video0", "video device to use")
	//flag.Parse()

	go wossamessa.RunWebCam("/dev/video1")
	wossamessa.Run()
}
