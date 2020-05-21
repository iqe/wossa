package wossamessa

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/blackjack/webcam"
)

const (
	V4L2_PIX_FMT_PJPG = 0x47504A50
	V4L2_PIX_FMT_YUYV = 0x56595559
)

type FrameSizes []webcam.FrameSize

func (slice FrameSizes) Len() int {
	return len(slice)
}

//For sorting purposes
func (slice FrameSizes) Less(i, j int) bool {
	ls := slice[i].MaxWidth * slice[i].MaxHeight
	rs := slice[j].MaxWidth * slice[j].MaxHeight
	return ls < rs
}

//For sorting purposes
func (slice FrameSizes) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

var supportedFormats = map[webcam.PixelFormat]bool{
	V4L2_PIX_FMT_PJPG: true,
	V4L2_PIX_FMT_YUYV: true,
}

// RunWebCam starts recording
func RunWebCam(dev string) {

	cam, err := webcam.Open(dev)
	if err != nil {
		panic(err.Error())
	}
	defer cam.Close()

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
		log.Println("No format found, exiting")
		return
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

	if size == nil {
		log.Println("No matching frame size, exiting")
		return
	}

	fmt.Fprintln(os.Stderr, "Requesting", formatDesc[format], size.GetString())
	f, w, h, err := cam.SetImageFormat(format, uint32(size.MaxWidth), uint32(size.MaxHeight))
	if err != nil {
		log.Println("SetImageFormat return error", err)
		return

	}
	fmt.Fprintf(os.Stderr, "Resulting image format: %s %dx%d\n", formatDesc[f], w, h)

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

	timeout := uint32(5) //5 seconds
	start := time.Now()

	for {
		err = cam.WaitForFrame(timeout)

		switch err.(type) {
		case nil:
		case *webcam.Timeout:
			log.Println(err)
			continue
		default:
			log.Println(err)
			return
		}

		frame, err := cam.ReadFrame()
		if err != nil {
			log.Println(err)
			return
		}
		if len(frame) != 0 {

			// Calculation
			contrast := 180
			factor := (259 * (contrast + 255)) / (255 * (259 - contrast))

			sum := 0
			sumx := 0
			for i := 0; i < len(frame); i += 2 {
				sum += int(frame[i])
				sumx += (int(frame[i]) * factor)
			}

			// Encoding
			if d := time.Since(start); d > 2*time.Second {

				log.Printf("Sum: %d\n", sum)
				log.Printf("Sumx: %d\n", sumx)

				select {
				case fi <- frame:
					<-back
				default:
				}
				start = time.Now()
			}
		}
		time.Sleep(75 * time.Millisecond)
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

			rgba := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))

			x := 0
			y := 0
			for i := 0; i < len(frame); i += 2 {
				luma := adjustPixel(frame[i], config.Contrast, config.Brightness)

				col := color.RGBA{R: luma, G: luma, B: luma, A: 255}

				if y == config.OffsetY || y == config.OffsetY+config.CaptureHeight {
					if x >= config.OffsetX && x <= config.OffsetX+config.CaptureWidth {
						col = color.RGBA{R: 255, A: 255}
					}
				}

				if x == config.OffsetX || x == config.OffsetX+config.CaptureWidth {
					if y >= config.OffsetY && y <= config.OffsetY+config.CaptureHeight {
						col = color.RGBA{R: 255, A: 255}
					}
				}

				rgba.Set(x, y, col)

				x++
				if x == int(w) {
					x = 0
					y++
				}
			}

			yuyv := image.NewYCbCr(image.Rect(0, 0, int(w), int(h)), image.YCbCrSubsampleRatio422)
			for i := range yuyv.Cb {
				ii := i * 4
				yuyv.Y[i*2] = frame[ii]
				yuyv.Y[i*2+1] = frame[ii+2]
				yuyv.Cb[i] = frame[ii+1]
				yuyv.Cr[i] = frame[ii+3]

			}
			img = rgba
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

func httpImage(addr string, li chan *bytes.Buffer) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("connect from", r.RemoteAddr, r.URL)
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		//remove stale image
		<-li

		img := <-li

		w.Header().Set("Content-Type", "image/jpeg")

		if _, err := w.Write(img.Bytes()); err != nil {
			log.Println(err)
			return
		}

	})

	log.Fatal(http.ListenAndServe(addr, nil))
}

func httpVideo(addr string, li chan *bytes.Buffer) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("connect from", r.RemoteAddr, r.URL)
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		//remove stale image
		<-li
		const boundary = `frame`
		w.Header().Set("Content-Type", `multipart/x-mixed-replace;boundary=`+boundary)
		multipartWriter := multipart.NewWriter(w)
		multipartWriter.SetBoundary(boundary)
		for {
			img := <-li
			image := img.Bytes()
			iw, err := multipartWriter.CreatePart(textproto.MIMEHeader{
				"Content-type":   []string{"image/jpeg"},
				"Content-length": []string{strconv.Itoa(len(image))},
			})
			if err != nil {
				log.Println(err)
				return
			}
			_, err = iw.Write(image)
			if err != nil {
				log.Println(err)
				return
			}
		}
	})

	log.Fatal(http.ListenAndServe(addr, nil))
}
