package main

import (
	"fmt"
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

	_, queue, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+".*",
		pubsub.Durable,
	)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}
	fmt.Printf("Queue %v declared and bound!\n", queue.Name)

	gamelogic.PrintServerHelp()

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "pause":
			log.Println("Sending pause message")
			payload := routing.PlayingState{
				IsPaused: true,
			}
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, payload)
			if err != nil {
				log.Println("Error publishing JSON:", err)
			}

		case "resume":
			log.Println("Sending resume message")
			payload := routing.PlayingState{
				IsPaused: false,
			}

			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, payload)
			if err != nil {
				log.Println("Error publishing JSON:", err)
			}

		case "help":
			gamelogic.PrintServerHelp()

		case "quit":
			log.Println("Quitting...")
			return

		default:
			log.Printf("Error unknown command: %s\n", input[0])
		}
	}
}
