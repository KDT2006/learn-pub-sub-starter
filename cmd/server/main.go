package main

import (
	"encoding/json"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	connStr := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	log.Printf("Successfully connected to the server %s\n", connStr)

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}

	gamelogic.PrintServerHelp()

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "pause":
			log.Println("Sending pause message")
			encoded, err := json.Marshal(routing.PlayingState{
				IsPaused: true,
			})
			if err != nil {
				log.Println("Error encoding PlayingState to JSON:", err)
				continue
			}

			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, encoded)
			if err != nil {
				log.Println("Error publishing JSON:", err)
			}

		case "resume":
			log.Println("Sending resume message")
			encoded, err := json.Marshal(routing.PlayingState{
				IsPaused: false,
			})
			if err != nil {
				log.Println("Error encoding PlayingState to JSON:", err)
				continue
			}

			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, encoded)
			if err != nil {
				log.Println("Error publishing JSON:", err)
			}

		case "quit":
			log.Println("Quitting...")
			return

		default:
			log.Printf("Error unknown command: %s", input[0])
		}
	}
}
