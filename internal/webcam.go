package wossamessa

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"sort"
	"time"

	"github.com/blackjack/webcam"
	log "github.com/inconshreveable/log15"
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
	for f := range formatDesc {
		if f == V4L2_PIX_FMT_YUYV {
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

	for _, f := range frames {
		log.Debug("Supported framesize", "format", formatDesc[format], "framezsize", f.GetString())
	}
	var size *webcam.FrameSize
	size = &frames[0]

	f, w, h, err := cam.SetImageFormat(format, uint32(size.MaxWidth), uint32(size.MaxHeight))
	if err != nil {
		return nil, 0, 0, 0, err
	}
	log.Info("Selected image format", "format", formatDesc[f], "w", w, "h", h)

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
func RunWebCam(dev string, calibrationValues chan int) error {
	log.Info("Using webcam", "device", dev)

	meterChanges := make(chan Meter)

	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("Loading config: %s", err)
	}

	mqttClient := NewMqttClient(meterChanges)
	err = mqttClient.Connect(config)
	if err != nil {
		return fmt.Errorf("Initial connect to MQTT: %s", err)
	}
	defer mqttClient.Disconnect()

	cam, f, w, h, err := initializeWebcam(dev)
	defer cam.Close()

	// start streaming
	err = cam.StartStreaming()
	if err != nil {
		return fmt.Errorf("Start streaming: %s", err)
	}

	var (
		fi           = make(chan []byte)
		copyComplete = make(chan bool)
	)
	go encodeToImage(cam, copyComplete, fi, w, h, f)

	log.Info("Started capturing")

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
			log.Debug("Timeout while reading next frame", "error", err)
			continue
		default:
			return fmt.Errorf("Unexpected error while reading next frame: %s", err)
		}
		// Calculation
		config, _ := loadConfig()

		sum, x, y := 0, 0, 0
		for i := 0; i < len(frame); i += 2 { // in YUYV, every second byte contains luma (greyscale pixel)
			// Calculate sum
			if y >= config.OffsetY && y <= config.OffsetY+config.CaptureHeight {
				if x >= config.OffsetX && x <= config.OffsetX+config.CaptureWidth {
					sum += int(frame[i])
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
				log.Warn("Failure while pulsing meter", "error", err)
			}

			log.Debug("Pulse detected", "meter", m)

			meterChanges <- m
			zeroingPending = true
			lastMeterChange = now
		}

		// Pulse reset
		if zeroingPending && now.Sub(lastMeterChange) > zeroingPeriod {
			m, err := ZeroPulseMeter()
			if err != nil {
				log.Warn("Failure while resetting meter to 0 l/m", "error", err)
			}

			log.Debug("Zeroing meter", "meter", m)

			meterChanges <- m
			zeroingPending = false
			lastMeterChange = now
		}

		if frameCount%20 == 0 { // ~ every 2 seconds
			// MQTT config changes
			if !mqttClient.UsesConfig(config) {
				err := mqttClient.Connect(config)
				if err != nil {
					return fmt.Errorf("Reconnect to MQTT: %s", err)
				}
			}

			// Brightness / Contrast config changes
			updateBrightnessAndContrast(cam, config)

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
				log.Warn("Encode to image: Error while loading config", "error", err)
				continue
			}
			createImage(frame, rgba, w, h, config)
		default:
			// If we reach this, that's a bug
			panic("Encode to image: Frame not in YUYV format")
		}

		//convert to jpeg (a lot faster than png)
		buf := &bytes.Buffer{}
		err := jpeg.Encode(buf, rgba, &jpeg.Options{Quality: 90})
		if err != nil {
			log.Warn("Encoding JPEG", "error", err)
			continue
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

func updateBrightnessAndContrast(cam *webcam.Webcam, config Config) {
	cidBrightness := webcam.ControlID(webcam.V4L2_CID_BASE)
	cidContrast := webcam.ControlID(webcam.V4L2_CID_BASE + 1)

	trySetCameraControl(cam, cidBrightness, "brightness", int32(config.Brightness))
	trySetCameraControl(cam, cidContrast, "contrast", int32(config.Contrast))
}

func trySetCameraControl(cam *webcam.Webcam, cid webcam.ControlID, name string, newValue int32) {
	controls := cam.GetControls()
	for id, c := range controls {
		if id == cid {
			newValue = clamp(newValue, c.Min, c.Max)
		}
	}

	oldValue, err := cam.GetControl(cid)
	if err != nil {
		log.Warn("Failed to get webcam control", "control", name, "error", err)
	} else {
		if newValue != oldValue {
			err = cam.SetControl(cid, newValue)
			if err != nil {
				log.Warn("Failed to set webcam control", "control", name, "oldValue", oldValue, "newValue", config.Brightness, "error", err)
			} else {
				log.Debug("Set webcam control", "control", name, "oldValue", oldValue, "newValue", newValue)
			}
		}
	}
}

func clamp(val int32, min int32, max int32) int32 {
	if val < min {
		val = min
	}
	if val > max {
		val = max
	}
	return val
}
