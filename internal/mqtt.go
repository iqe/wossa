package wossamessa

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	log "github.com/inconshreveable/log15"

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

	log.Info("Connecting to MQTT broker", "broker", broker)

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
		log.Info("Disconnecting from MQTT broker")

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
			err := c.sendMeterMessage(m)
			if err != nil {
				log.Warn("Failed to send meter message", "error", err)
			}
			c.resetTicker()
		case v := <-c.calibrationValues:
			err := c.sendCalibrationMessage(v)
			if err != nil {
				log.Warn("Failed to send calibration message", "error", err)
			}
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

func (c *mqttClient) sendMeterMessage(meter Meter) error {
	return c.publish(c.config.MqttTopic, meter)
}

func (c *mqttClient) sendCalibrationMessage(value int) error {
	return c.publish(c.config.MqttCalibrationTopic, value)
}

func (c *mqttClient) publish(topic string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	log.Debug("Publishing MQTT message", "topic", topic, "message", string(data))

	// Ignore result of publish so we don't slow down the capturing
	c.client.Publish(topic, 0, false, data)
	return nil

	// token := c.client.Publish(topic, 0, false, data)
	// token.Wait()
	// return token.Error()
}

func (c *mqttClient) resetTicker() {
	c.ticker = time.NewTicker(c.tickerPeriod)
}
