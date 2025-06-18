package main

import (
	"log"
	"os"
	"os/signal"

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

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	log.Println("Shutting down...")
}
