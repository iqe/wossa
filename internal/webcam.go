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

func initializeWebcam(dev string) (*webcam.Webcam, webcam.PixelFormat, uint32, uint32, error) {
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

	return cam, f, w, h, nil
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

	cam, f, w, h, err := initializeWebcam(dev)
	defer cam.Close()

	err = initializeMqttCommunication(meterChanges, calibrationValues)
	if err != nil {
		panic(err.Error())
	}

	// start streaming
	err = cam.StartStreaming()
	if err != nil {
		log.Println(err)
		return
	}

	var (
		fi   = make(chan []byte)
		back = make(chan struct{})
	)
	go encodeToImage(cam, back, fi, w, h, f)

	detector := pulseDetector{}
	zeroingPending := false
	lastMeterChange := time.Now()
	lastMessageSent := time.Now()
	frameCount := 0
	for {
		s := time.Now()
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
		sum := 0
		for i := 0; i < len(frame); i += 2 {
			original := frame[i]
			adjusted := adjustPixel(original, config.Contrast, config.Brightness)

			frame[i] = adjusted
			sum += int(adjusted)
		}

		// Pulse detection
		now := time.Now()
		pulseDetected := detector.process(sum)

		messageSent := false

		if pulseDetected {
			m, _ := loadMeter()
			m.Liters += config.StepSize
			m.LitersPerMinute = float64(config.StepSize) / now.Sub(lastMeterChange).Minutes()
			m.Timestamp = now.Unix()
			saveMeter(m)
			meterChanges <- m
			log.Printf("Pulse detected l=%d lpm=%f\n", m.Liters, m.LitersPerMinute)
			zeroingPending = true
			lastMeterChange = now
			lastMessageSent = now
			messageSent = true
		}
		// Pulse reset
		if zeroingPending && now.Sub(lastMeterChange) > 5*time.Second {
			m, _ := loadMeter()
			m.LitersPerMinute = 0
			m.Timestamp = now.Unix()
			saveMeter(m)
			meterChanges <- m
			log.Println("Zeroing")
			zeroingPending = false
			lastMeterChange = now
			lastMessageSent = now
			messageSent = true
		}

		if now.Sub(lastMeterChange) > 10*time.Second {
			lastMeterChange = now
		}

		if !messageSent && now.Sub(lastMessageSent) > 15*time.Second {
			m, _ := loadMeter()
			m.Timestamp = now.Unix()
			meterChanges <- m
			messageSent = true
			lastMessageSent = now
		}

		// Preview
		if frameCount%20 == 0 { // ~ every 2 seconds
			select {
			case fi <- frame:
				<-back
			default:
			}
		}

		if config.Calibration && frameCount%5 == 0 { // ~ 2 msg/s
			calibrationValues <- sum
		}

		frameCount++
		log.Println(frameCount)
		// Aim for ~ 10fps
		//time.Sleep(75 * time.Millisecond)

		fmt.Printf("%v", time.Now().Sub(s))
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

func encodeToImage(wc *webcam.Webcam, back chan struct{}, fi chan []byte, w, h uint32, format webcam.PixelFormat) {
	var (
		frame []byte
		img   image.Image
	)

	for {
		bframe := <-fi
		// copy frame
		if len(frame) < len(bframe) {
			frame = make([]byte, len(bframe))
		}
		copy(frame, bframe)
		back <- struct{}{}

		switch format {
		case V4L2_PIX_FMT_YUYV:
			config, err := loadConfig()
			if err != nil {
				return
			}
			img = createImage(frame, int(w), int(h), config)
		default:
			log.Fatal("invalid format ?")
		}

		//convert to jpeg
		buf := &bytes.Buffer{}
		if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 100}); err != nil {
			log.Fatal(err)
			return
		}
		savePreview(buf.Bytes())
	}
}

func createImage(frame []byte, w int, h int, config Config) *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))

	x := 0
	y := 0
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

	return rgba
}
