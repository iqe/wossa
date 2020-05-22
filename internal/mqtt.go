package wossamessa

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

/* MQTT communication */

func initializeMqttCommunication(meterChanges chan Meter, calibrationValues chan int) error {
	config, _ := loadConfig()

	broker := fmt.Sprintf("tcp://%s:%d", config.MqttHost, config.MqttPort)
	log.Printf("MQTT broker: %s\n", broker)
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(fmt.Sprintf("wossamessa-%d", rand.Int31()))

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	go sendToMqtt(client, meterChanges, calibrationValues)
	m, _ := loadMeter()
	meterChanges <- m
	return nil
}

func sendToMqtt(client mqtt.Client, meterChanges chan Meter, calibrationValues chan int) {
	for {
		select {
		case meter := <-meterChanges:
			config, _ := loadConfig()
			data, _ := json.Marshal(meter)
			log.Printf("Mqtt: Sending meter: %s\n", data)

			token := client.Publish(config.MqttTopic, 0, false, data)
			token.Wait()
		case cal := <-calibrationValues:
			config, _ := loadConfig()
			data, _ := json.Marshal(cal)
			log.Printf("Mqtt: Sending calibration value: %s\n", cal)
			token := client.Publish(config.MqttTopic, 0, false, data)
			token.Wait()
		}
	}
}
