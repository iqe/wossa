package wossa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	log "github.com/inconshreveable/log15"
)

// Meter represents the current value of the meter
type Meter struct {
	Liters          int     `json:"l"`
	LitersPerMinute float64 `json:"lpm"`
	Timestamp       int64   `json:"ts"`
}

// Config represents the configuration
type Config struct {
	CaptureWidth      int    `json:"capture-w"`
	CaptureHeight     int    `json:"capture-h"`
	OffsetX           int    `json:"offset-x"`
	OffsetY           int    `json:"offset-y"`
	TriggerHigh       int    `json:"trigger-high"`
	TriggerLow        int    `json:"trigger-low"`
	Contrast          int    `json:"contrast"`
	Brightness        int    `json:"brightness"`
	StepSize          int    `json:"step-size"`
	ZeroingSeconds    int    `json:"zeroing-seconds"`
	MqttHost          string `json:"mqtt-host"`
	MqttPort          int    `json:"mqtt-port"`
	MqttTopic         string `json:"mqtt-topic"`
	MqttTickerSeconds int    `json:"mqtt-ticker-seconds"`
	Calibration       bool   `json:"calibration"`
}

var meterLoaded = false

var config = Config{
	CaptureWidth:      10,
	CaptureHeight:     30,
	OffsetX:           5,
	OffsetY:           15,
	Contrast:          128,
	Brightness:        128,
	MqttHost:          "zap",
	MqttPort:          1883,
	MqttTopic:         "wossa/meter",
	MqttTickerSeconds: 300,
	Calibration:       false,
	StepSize:          1,
	TriggerHigh:       1_000_000,
	TriggerLow:        500_000,
	ZeroingSeconds:    60,
}
var configLoaded = false

var meter = Meter{}
var lastMeterSave = time.Now()
var preview = make([]byte, 0)

// ConfigDir is the directory where config and data files are stored.
var ConfigDir = "."

func saveConfig(cfg Config) error {
	config = cfg

	err := saveToFile(cfg, "config.json")
	if err != nil {
		return err
	}

	return nil
}

func loadConfig() (Config, error) {
	if configLoaded {
		return config, nil
	}

	err := loadFromFile(&config, "config.json")
	if err == nil {
		configLoaded = true
	}
	return config, err
}

func PulseMeter() (Meter, error) {
	now := time.Now()

	m, _ := loadMeter()
	lastMeterChange := time.Unix(m.Timestamp, 0)

	m.Liters += config.StepSize
	m.LitersPerMinute = float64(config.StepSize) / now.Sub(lastMeterChange).Minutes()
	m.Timestamp = now.Unix()

	err := saveMeter(m, false)
	return m, err
}

func ZeroPulseMeter() (Meter, error) {
	m, _ := loadMeter()
	m.LitersPerMinute = 0
	m.Timestamp = time.Now().Unix()
	err := saveMeter(m, false)

	return m, err
}

func UpdateMeter(m Meter) (Meter, error) {
	err := saveMeter(m, true)
	return m, err
}

func PersistMeter() error {
	return saveMeter(meter, true)
}

func saveMeter(m Meter, forceSaveToDisk bool) error {
	meter = m

	// Save to disk if requested or if the meter stops running or after 5 min
	if forceSaveToDisk || m.LitersPerMinute == 0 || time.Now().Sub(lastMeterSave) > 5*time.Minute {
		err := saveToFile(m, "meter.json")
		if err != nil {
			return fmt.Errorf("Saving meter: %s\n", err)
		}
		lastMeterSave = time.Now()
	}

	return nil
}

func loadMeter() (Meter, error) {
	if meterLoaded {
		return meter, nil
	}

	err := loadFromFile(&meter, "meter.json")
	if err == nil {
		meterLoaded = true
	}
	return meter, err
}

func saveToFile(v interface{}, filename string) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("Marshalling JSON for %s: %s\n", filename, err)
	}
	err = ioutil.WriteFile(ConfigDir+"/"+filename, data, 0644)
	if err != nil {
		return fmt.Errorf("Writing %s: %s\n", filename, err)
	}

	log.Info("Saved file", "filename", filename, "content", v)

	return nil
}

func loadFromFile(v interface{}, filename string) error {
	data, err := ioutil.ReadFile(ConfigDir + "/" + filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Reading %s: %s\n", filename, err)
		}
		// if file does not exist, just use the defaults
	} else {
		err = json.Unmarshal(data, v)
		if err != nil {
			return fmt.Errorf("Parsing %s: %s\n", filename, err)
		}
	}

	return nil
}

func savePreview(jpeg []byte) error {
	preview = jpeg
	return nil
}

func loadPreview() ([]byte, error) {
	return preview, nil
}
