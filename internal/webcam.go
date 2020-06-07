package wossamessa

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"os"
	"sort"
	"time"

	"github.com/blackjack/webcam"
)

const (
	V4L2_PIX_FMT_YUYV = 0x56595559
)

type FrameSizes []webcam.FrameSize

func (slice FrameSizes) Len() int {
	return len(slice)
}

// For sorting purposes
func (slice FrameSizes) Less(i, j int) bool {
	ls := slice[i].MaxWidth * slice[i].MaxHeight
	rs := slice[j].MaxWidth * slice[j].MaxHeight
	return ls < rs
}

// For sorting purposes
func (slice FrameSizes) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func initializeWebcam(dev string) (*webcam.Webcam, webcam.PixelFormat, int, int, error) {
	cam, err := webcam.Open(dev)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	// select pixel format
	formatDesc := cam.GetSupportedFormats()

	var format webcam.PixelFormat
	for f, s := range formatDesc {
		if f == V4L2_PIX_FMT_YUYV {
			log.Printf("Using format %s\n", s)
			format = f
			break
		}
	}
	if format == 0 {
		return nil, 0, 0, 0, fmt.Errorf("Webcam does not support YUYV format")
	}

	// select frame size
	frames := FrameSizes(cam.GetSupportedFrameSizes(format))
	sort.Sort(frames)

	fmt.Fprintln(os.Stderr, "Supported frame sizes for format", formatDesc[format])
	for _, f := range frames {
		fmt.Fprintln(os.Stderr, f.GetString())
	}
	var size *webcam.FrameSize
	size = &frames[0]

	fmt.Fprintln(os.Stderr, "Requesting", formatDesc[format], size.GetString())
	f, w, h, err := cam.SetImageFormat(format, uint32(size.MaxWidth), uint32(size.MaxHeight))
	if err != nil {
		return nil, 0, 0, 0, err
	}
	fmt.Fprintf(os.Stderr, "Resulting image format: %s %dx%d\n", formatDesc[f], w, h)

	err = cam.SetBufferCount(16)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	err = cam.SetFps(10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SetFps failed")
		return nil, 0, 0, 0, err
	}

	return cam, f, int(w), int(h), nil
}

func readNextFrame(cam *webcam.Webcam) ([]byte, error) {
	timeout := uint32(5) //5 seconds
	err := cam.WaitForFrame(timeout)
	if err != nil {
		return []byte{}, err
	}

	frame, err := cam.ReadFrame()
	if err != nil {
		return []byte{}, err
	}

	if len(frame) == 0 {
		return []byte{}, fmt.Errorf("Webcam returned empty frame")
	}

	return frame, err
}

// RunWebCam starts recording
func RunWebCam(dev string) {
	meterChanges := make(chan Meter)
	calibrationValues := make(chan int)

	config, err := loadConfig()
	if err != nil {
		panic(err.Error())
	}

	mqttClient := NewMqttClient(meterChanges, calibrationValues)
	err = mqttClient.Connect(config)
	if err != nil {
		panic(err.Error())
	}
	defer mqttClient.Disconnect()

	cam, f, w, h, err := initializeWebcam(dev)
	defer cam.Close()

	// start streaming
	err = cam.StartStreaming()
	if err != nil {
		log.Println(err)
		return
	}

	var (
		fi           = make(chan []byte)
		copyComplete = make(chan bool)
	)
	go encodeToImage(cam, copyComplete, fi, w, h, f)

	zeroingPeriod := time.Duration(config.ZeroingSeconds) * time.Second

	detector := pulseDetector{}
	zeroingPending := false
	lastMeterChange := time.Now()
	frameCount := 0
	for {
		frame, err := readNextFrame(cam)
		switch err.(type) {
		case nil:
		case *webcam.Timeout:
			log.Println(err)
			continue
		default:
			log.Println(err)
			return
		}
		// Calculation
		config, _ := loadConfig()

		sum, x, y := 0, 0, 0
		for i := 0; i < len(frame); i += 2 { // in YUYV, every second byte contains luma (greyscale pixel)
			original := frame[i]

			// Brightness / contrast correction
			adjusted := adjustPixel(original, config.Contrast, config.Brightness)
			frame[i] = adjusted

			// Calculate sum
			if y >= config.OffsetY && y <= config.OffsetY+config.CaptureHeight {
				if x >= config.OffsetX && x <= config.OffsetX+config.CaptureWidth {
					sum += int(adjusted)
				}
			}

			// Track x, y coordinates
			x++
			if x == int(w) {
				x = 0
				y++
			}
		}

		// Pulse detection
		now := time.Now()
		pulseDetected := detector.process(sum)

		if pulseDetected {
			m, err := PulseMeter()
			if err != nil {
				log.Printf("Failure while pulsing meter: %s\n", err)
			}

			meterChanges <- m
			zeroingPending = true
			lastMeterChange = now
		}

		// Pulse reset
		if zeroingPending && now.Sub(lastMeterChange) > zeroingPeriod {
			m, err := ZeroPulseMeter()
			if err != nil {
				log.Printf("Failure while resetting meter to 0 l/m: %s\n", err)
			}

			meterChanges <- m
			zeroingPending = false
			lastMeterChange = now
		}

		if frameCount%20 == 0 { // ~ every 2 seconds
			// MQTT config changes
			if !mqttClient.UsesConfig(config) {
				err := mqttClient.Connect(config)
				if err != nil {
					log.Fatalf("Failed to connect to MQTT broker: %s\n", err)
				}
			}

			// Preview
			select {
			case fi <- frame:
				<-copyComplete
			default:
			}
		}

		if config.Calibration && frameCount%5 == 0 { // ~ 2 msg/s
			calibrationValues <- sum
		}

		frameCount++
	}
}

func adjustPixel(pixel byte, contrast int, brightness int) byte {
	c := float32(contrast) - 128
	b := float32(brightness) - 128

	c = (259 * (c + 255)) / (255 * (259 - c))

	p := c*(float32(pixel)-128) + 128 + b

	if p < 0 {
		p = 0
	}
	if p > 255 {
		p = 255
	}
	return byte(p)
}

func encodeToImage(wc *webcam.Webcam, copyComplete chan bool, fi chan []byte, w, h int, format webcam.PixelFormat) {
	frame := make([]byte, w*h*2) // *2 because frame is in YUYV format
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))

	for {
		bframe := <-fi
		copy(frame, bframe)
		copyComplete <- true

		switch format {
		case V4L2_PIX_FMT_YUYV:
			config, err := loadConfig()
			if err != nil {
				return
			}
			createImage(frame, rgba, w, h, config)
		default:
			log.Fatal("invalid format ?")
		}

		//convert to jpeg (a lot faster than png)
		buf := &bytes.Buffer{}
		err := jpeg.Encode(buf, rgba, &jpeg.Options{Quality: 90})
		if err != nil {
			log.Fatal(err)
		}
		savePreview(buf.Bytes())
	}
}

func createImage(frame []byte, rgba *image.RGBA, w int, h int, config Config) {
	x, y := 0, 0
	for i := 0; i < len(frame); i += 2 {
		luma := frame[i]
		col := color.RGBA{R: luma, G: luma, B: luma}

		// Draw red border around capture area
		if y == config.OffsetY || y == config.OffsetY+config.CaptureHeight {
			if x >= config.OffsetX && x <= config.OffsetX+config.CaptureWidth {
				col = color.RGBA{R: 255}
			}
		}

		if x == config.OffsetX || x == config.OffsetX+config.CaptureWidth {
			if y >= config.OffsetY && y <= config.OffsetY+config.CaptureHeight {
				col = color.RGBA{R: 255}
			}
		}

		rgba.Set(x, y, col)

		x++
		if x == int(w) {
			x = 0
			y++
		}
	}
}
