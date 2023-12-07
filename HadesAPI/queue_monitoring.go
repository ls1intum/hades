package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Mtze/HadesCI/shared/payload"
	log "github.com/sirupsen/logrus"
)

const monitoring_url = "http://%s:15672/api/queues/%%2F/%s" // %%2F is url encoded `/` for default vhost

type MonitoringValues struct {
	MessageSize  int                    `json:"message_size"`
	ConsumerSize int                    `json:"consumer_size"`
	Messages     []payload.QueuePayload `json:"messages"`
}

type MonitoringClient struct {
	host string
	user string
	pass string
}

func NewMonitoringClient(host, user, pass string) (*MonitoringClient, error) {
	u, err := url.Parse(host)
	if err != nil {
		log.WithError(err).Error("error parsing RabbitMQ URL")
		return nil, err
	}
	return &MonitoringClient{u.Host, user, pass}, nil
}

func (m *MonitoringClient) getSizes() (message_size, consumer_size int) {
	url := fmt.Sprintf(monitoring_url, m.host, "builds")
	log.Debug("Getting queue size from ", url)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(m.user, m.pass)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("error getting queue size")
		return -1, -1
	}
	defer resp.Body.Close()
	// Decode the JSON response into a struct
	var queueInfo struct {
		Messages  int `json:"messages"`
		Consumers int `json:"consumers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&queueInfo); err != nil {
		log.WithError(err).Error("error decoding queue information")
		return -1, -1
	}
	return queueInfo.Messages, queueInfo.Consumers
}

func (m *MonitoringClient) GetQueueState() MonitoringValues {
	msg_size, cons_size := m.getSizes()
	// No messages in queue, save extra request
	if msg_size < 1 {
		return MonitoringValues{
			MessageSize:  msg_size,
			ConsumerSize: cons_size,
		}
	}
	url := fmt.Sprintf(monitoring_url+"/get", m.host, "builds")

	req_payload := struct {
		Count    int    `json:"count"`
		AckMode  string `json:"ackmode"`
		Encoding string `json:"encoding"`
	}{
		Count:    msg_size,
		AckMode:  "ack_requeue_true",
		Encoding: "auto",
	}

	// Marshal the struct into JSON
	jsonValue, err := json.Marshal(req_payload)
	if err != nil {
		log.Fatal(err)
	}

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	req.SetBasicAuth(m.user, m.pass)
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("error getting queue size")
		return MonitoringValues{}
	}
	defer resp.Body.Close()
	// Decode the JSON response into a struct
	var messages []struct {
		PayloadString string `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		log.WithError(err).Error("error decoding queue information")
		return MonitoringValues{}
	}
	// Extracting just the payloads
	var payloads []payload.QueuePayload
	for _, message := range messages {
		var payload payload.QueuePayload
		err := json.Unmarshal([]byte(message.PayloadString), &payload)
		if err != nil {
			log.WithError(err).Error("error decoding queue information")
			continue
		}
		// Filter confidential information
		payload.Metadata = map[string]string{}
		for i := range payload.Steps {
			payload.Steps[i].Metadata = map[string]string{}
		}
		payloads = append(payloads, payload)
	}
	return MonitoringValues{
		MessageSize:  msg_size,
		ConsumerSize: cons_size,
		Messages:     payloads,
	}
}
