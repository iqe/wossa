package wossamessa

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"
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
		//log.Printf("Failed to read config.json: %s\n", err)
		return config, nil
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Failed to parse config.json: %s\n", err)
		return config, nil
	}

	configLoaded = true
	return config, nil
}

func PulseMeter() Meter {
	now := time.Now()

	m, _ := loadMeter()
	lastMeterChange := time.Unix(m.Timestamp, 0)

	m.Liters += config.StepSize
	m.LitersPerMinute = float64(config.StepSize) / now.Sub(lastMeterChange).Minutes()
	m.Timestamp = now.Unix()

	log.Printf("Pulse %v\n", m)

	saveMeter(m)
	return m
}

func ZeroPulseMeter() Meter {
	m, _ := loadMeter()
	m.LitersPerMinute = 0
	m.Timestamp = time.Now().Unix()
	saveMeter(m)

	log.Printf("Zero %v\n", m)

	return m
}

func saveMeter(m Meter) error {
	meter = m

	// Save to disk whenever the meter stops running or after 5 min
	if m.LitersPerMinute == 0 || time.Now().Sub(lastMeterSave) > 5*time.Minute {
		err := saveToFile(m, "meter.json")
		if err != nil {
			return err
		}
		lastMeterSave = time.Now()
	}

	return nil
}

func saveToFile(v interface{}, filename string) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(ConfigDir+"/"+filename, data, 0644)
	if err != nil {
		return err
	}
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
