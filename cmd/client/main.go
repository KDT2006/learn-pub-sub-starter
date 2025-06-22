package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

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

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatal(err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}

	pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username,
		routing.PauseKey,
		pubsub.Transient,
	)

	state := gamelogic.NewGameState(username)
	pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username,
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(state),
	)

	pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		"army_moves."+username,
		"army_moves.*",
		pubsub.Transient,
		handlerMove(state, ch),
	)

	pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		"war",
		routing.WarRecognitionsPrefix+".*",
		pubsub.Durable,
		handlerWar(state, ch),
	)

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "spawn":
			state.CommandSpawn(input)

		case "move":
			if move, err := state.CommandMove(input); err == nil {
				log.Println("Move successful")
				err = pubsub.PublishJSON(
					ch,
					string(routing.ExchangePerilTopic),
					"army_moves."+username,
					move,
				)
				if err != nil {
					log.Println("Error publishing JSON:", err)
				}
			}

		case "status":
			state.CommandStatus()

		case "help":
			gamelogic.PrintClientHelp()

		case "spam":
			if len(input) != 2 {
				log.Printf("Error invalid usage of spam. Usage: spam <iterations>")
				continue
			}

			n, err := strconv.Atoi(input[1])
			if err != nil {
				log.Printf("Error converting n to int: %v\n", err)
				continue
			}

			for range n {
				ml := gamelogic.GetMaliciousLog()
				message := routing.GameLog{
					CurrentTime: time.Now(),
					Message:     ml,
					Username:    username,
				}
				err = pubsub.PublishGob(
					ch,
					routing.ExchangePerilTopic,
					routing.GameLogSlug+"."+username,
					message,
				)
				if err != nil {
					log.Printf("Error Publishing GOB: %v\n", err)
				}
			}

		case "quit":
			gamelogic.PrintQuit()
			return

		default:
			log.Printf("Error unknown command: %s\n", input[0])
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(ps routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")

		gs.HandlePause(ps)

		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(mv gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")

		switch gs.HandleMove(mv) {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			warRecognition := gamelogic.RecognitionOfWar{
				Attacker: mv.Player,
				Defender: gs.GetPlayerSnap(),
			}

			routingKey := routing.WarRecognitionsPrefix + "." + mv.Player.Username
			err := pubsub.PublishJSON(
				ch,
				string(routing.ExchangePerilTopic),
				routingKey,
				warRecognition,
			)

			if err != nil {
				log.Println("Error publishing war recognition:", err)
				return pubsub.NackRequeue
			}

			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}

	}
}

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(row gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")

		outcome, winner, loser := gs.HandleWar(row)

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			message := fmt.Sprintf("%s won a war against %s", winner, loser)
			err := pubsub.PublishGob(
				ch,
				routing.ExchangePerilTopic,
				routing.GameLogSlug+"."+row.Attacker.Username,
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     message,
					Username:    gs.GetUsername(),
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}

			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			message := fmt.Sprintf("%s won a war against %s", winner, loser)
			err := pubsub.PublishGob(
				ch,
				routing.ExchangePerilTopic,
				routing.GameLogSlug+"."+row.Attacker.Username,
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     message,
					Username:    gs.GetUsername(),
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}

			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			message := fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			err := pubsub.PublishGob(
				ch,
				routing.ExchangePerilTopic,
				routing.GameLogSlug+"."+row.Attacker.Username,
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     message,
					Username:    gs.GetUsername(),
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}

			return pubsub.Ack
		default:
			log.Println("Error unknown outcome:", outcome)
			return pubsub.NackDiscard
		}
	}
}
