package wossamessa

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
	MqttHost             string `json:"mqtt-host"`
	MqttPort             int    `json:"mqtt-port"`
	MqttTopic            string `json:"mqtt-topic"`
	MqttCalibrationTopic string `json:"mqtt-calibration-topic"`
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
	MqttHost:             "localhost",
	MqttPort:             1883,
	MqttTopic:            "wossamessa/meter",
	MqttCalibrationTopic: "wossamessa/calibration",
	Flashlight:           false,
	Calibration:          false,
}
var meter = Meter{}

var preview = make([]byte, 0)

func saveConfig(cfg Config) error {
	config = cfg
	return nil
}

func loadConfig() (Config, error) {
	return config, nil
}

func saveMeter(m Meter) error {
	meter = m
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
