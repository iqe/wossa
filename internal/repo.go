package wossamessa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	CaptureWidth         int    `json:"capture-w"`
	CaptureHeight        int    `json:"capture-h"`
	OffsetX              int    `json:"offset-x"`
	OffsetY              int    `json:"offset-y"`
	TriggerHigh          int    `json:"trigger-high"`
	TriggerLow           int    `json:"trigger-low"`
	Contrast             int    `json:"contrast"`
	Brightness           int    `json:"brightness"`
	StepSize             int    `json:"step-size"`
	ZeroingSeconds       int    `json:"zeroing-seconds"`
	MqttHost             string `json:"mqtt-host"`
	MqttPort             int    `json:"mqtt-port"`
	MqttTopic            string `json:"mqtt-topic"`
	MqttCalibrationTopic string `json:"mqtt-calibration-topic"`
	MqttTickerSeconds    int    `json:"mqtt-ticker-seconds"`
	Flashlight           bool   `json:"flashlight"`
	Calibration          bool   `json:"calibration"`
}

var config = Config{
	CaptureWidth:         10,
	CaptureHeight:        30,
	OffsetX:              5,
	OffsetY:              15,
	Contrast:             128,
	Brightness:           128,
	MqttHost:             "zap",
	MqttPort:             1883,
	MqttTopic:            "wossamessa/meter",
	MqttCalibrationTopic: "wossamessa/calibration",
	MqttTickerSeconds:    300,
	Flashlight:           false,
	Calibration:          false,
	StepSize:             1,
	TriggerHigh:          1_000_000,
	TriggerLow:           500_000,
	ZeroingSeconds:       60,
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
	data, err := ioutil.ReadFile(ConfigDir + "/config.json")
	if err != nil {
		return config, fmt.Errorf("Reading config.json: %s\n", err)
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("Parsing config.json: %s\n", err)
	}

	configLoaded = true
	return config, nil
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

func loadMeter() (Meter, error) {
	return meter, nil
}

func savePreview(jpeg []byte) error {
	preview = jpeg
	return nil
}

func loadPreview() ([]byte, error) {
	return preview, nil
}
