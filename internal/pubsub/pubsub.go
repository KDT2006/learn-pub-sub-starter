package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type simpleQueueType byte

const (
	Durable = iota
	Transient
)

type Acktype byte

const (
	Ack = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	encoded, err := json.Marshal(val)
	if err != nil {
		return err
	}

	ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        encoded,
	})

	return nil
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(val)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        b.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	table := make(amqp.Table)
	table["x-dead-letter-exchange"] = "peril_dlx"
	queue, err := ch.QueueDeclare(
		queueName,
		queueType == Durable,
		queueType == Transient,
		queueType == Transient,
		false,
		table,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = ch.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return ch, queue, nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType,
	handler func(T) Acktype,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return fmt.Errorf("could not subscribe to pause: %v", err)
	}
	fmt.Printf("Queue %v declared and bound!\n", queue.Name)

	deliveryCh, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for delivery := range deliveryCh {
			var val T
			if err := json.Unmarshal(delivery.Body, &val); err != nil {
				fmt.Printf("Error unmarshalling message: %v\n", err)
				continue
			}

			ackt := handler(val)
			switch ackt {
			case Ack:
				log.Println("Ack handled")
				delivery.Ack(false)
			case NackRequeue:
				log.Println("Nack with requeue handled")
				delivery.Nack(false, true)
			case NackDiscard:
				log.Println("Nack with discard handled")
				delivery.Nack(false, false)
			}

			if err := delivery.Ack(false); err != nil {
				fmt.Printf("Error acknowledging message: %v\n", err)
			}
		}
	}()

	return nil
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType,
	handler func(T) Acktype,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return fmt.Errorf("could not subscribe to pause: %v", err)
	}
	fmt.Printf("Queue %v declared and bound!\n", queue.Name)

	err = ch.Qos(10, 0, true)
	if err != nil {
		return err
	}

	deliveryCh, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for message := range deliveryCh {
			var val T
			reader := bytes.NewReader(message.Body)
			if err := gob.NewDecoder(reader).Decode(&val); err != nil {
				fmt.Printf("Error decoding message: %v\n", err)
				continue
			}

			ackt := handler(val)
			switch ackt {
			case Ack:
				log.Println("Ack handled")
				message.Ack(false)
			case NackRequeue:
				log.Println("Nack with requeue handled")
				message.Nack(false, true)
			case NackDiscard:
				log.Println("Nack with discard handled")
				message.Nack(false, false)
			}
		}
	}()

	return nil
}
