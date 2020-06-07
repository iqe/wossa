package wossamessa

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type mqttClient struct {
	config            Config
	meterChanges      chan Meter
	calibrationValues chan int
	tickerPeriod      time.Duration
	client            mqtt.Client
	ticker            *time.Ticker
	ctx               context.Context
	ctxCancelFunc     context.CancelFunc
	connected         bool
}

func NewMqttClient(meterChanges chan Meter, calibrationValues chan int) *mqttClient {
	c := new(mqttClient)

	c.meterChanges = meterChanges
	c.calibrationValues = calibrationValues
	c.connected = false

	return c
}

func (c *mqttClient) Connect(config Config) error {
	c.Disconnect()

	broker := fmt.Sprintf("tcp://%s:%d", config.MqttHost, config.MqttPort)
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(fmt.Sprintf("wossamessa-%d", rand.Int31()))

	c.client = mqtt.NewClient(opts)
	c.config = config
	c.tickerPeriod = time.Duration(config.MqttTickerSeconds) * time.Second

	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	c.ctx, c.ctxCancelFunc = context.WithCancel(context.Background())
	c.resetTicker()
	go c.processMessages()

	c.connected = true
	return nil
}

func (c *mqttClient) Disconnect() {
	if c.connected {
		c.ticker.Stop()
		c.ctxCancelFunc()
		c.client.Disconnect(0)
		c.connected = false
	}
}

func (c *mqttClient) UsesConfig(newConfig Config) bool {
	curConfig := c.config

	sameHost := curConfig.MqttHost == newConfig.MqttHost
	samePort := curConfig.MqttPort == newConfig.MqttPort
	sameTopic := curConfig.MqttTopic == newConfig.MqttTopic
	sameCalTopic := curConfig.MqttCalibrationTopic == newConfig.MqttCalibrationTopic
	sameTicker := c.tickerPeriod == time.Duration(newConfig.MqttTickerSeconds)*time.Second

	return sameHost && samePort && sameTopic && sameCalTopic && sameTicker
}

func (c *mqttClient) processMessages() {
	for {
		select {
		case m := <-c.meterChanges:
			c.sendMeterMessage(m)
			c.resetTicker()
		case v := <-c.calibrationValues:
			c.sendCalibrationMessage(v)
			c.resetTicker()
		case <-c.ticker.C:
			m, err := loadMeter()
			if err == nil {
				m.LitersPerMinute = 0
				m.Timestamp = time.Now().Unix()
				c.sendMeterMessage(m)
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *mqttClient) sendMeterMessage(meter Meter) {
	config, _ := loadConfig()
	data, _ := json.Marshal(meter)
	log.Printf("Mqtt: Sending meter: %s\n", data)

	token := c.client.Publish(config.MqttTopic, 0, false, data)
	token.Wait()
}

func (c *mqttClient) sendCalibrationMessage(value int) {
	data, _ := json.Marshal(value)
	log.Printf("Mqtt: Sending calibration value: %s\n", value)
	token := c.client.Publish(c.config.MqttCalibrationTopic, 0, false, data)
	token.Wait()
}

func (c *mqttClient) resetTicker() {
	c.ticker = time.NewTicker(c.tickerPeriod)
}
