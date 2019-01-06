package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/BlacksunLabs/drgero/event"
	"github.com/BlacksunLabs/drgero/mq"
)

// Message is just a placeholder for the slack post body
type Message struct {
	Text string `json:"text"`
}

var whURL string
var connectString string

var m = new(mq.Client)

func post(message string) error {
	payload := Message{Text: fmt.Sprintf("Interesting paste found: https://pastebin.com/%s", message)}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("unable to marshal payload: %v", err)
		return err
	}

	_, err = http.Post(whURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("failed to post to slack")
		return err
	}

	return nil
}

func main() {
	whURL = os.Getenv("SLACK_WEBHOOK")
	connectString = os.Getenv("DG_CONNECT")
	err := m.Connect(connectString)
	if err != nil {
		fmt.Printf("unable to connect to RabbitMQ : %v", err)
	}

	queueName, err := m.NewTempQueue()
	if err != nil {
		fmt.Printf("could not create temporary queue : %v", err)
	}

	err = m.BindQueueToExchange(queueName, "events")
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	ch, err := m.GetChannel()
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	events, err := ch.Consume(
		queueName,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Printf("failed to register consumer to %s : %v", queueName, err)
		return
	}

	forever := make(chan bool)

	go func() {
		for e := range events {
			var event = new(event.Event)
			var err = json.Unmarshal(e.Body, event)
			if err != nil {
				fmt.Printf("failed to unmarshal event: %v", err)
				<-forever
			}

			if event.UserAgent != "psbmon" {
				break
			}

			pasteID := event.Message[1 : len(event.Message)-1]
			err = post(pasteID)
			if err != nil {
				log.Printf("%v", err)
			}
		}
	}()

	fmt.Println("[i] Waiting for events. To exit press CTRL+C")
	<-forever
}
