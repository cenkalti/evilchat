package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/cenkalti/envconfig"
	"github.com/streadway/amqp"
	"gopkg.in/igm/sockjs-go.v2/sockjs"
)

var dev = flag.Bool("dev", false, "use development config")

const (
	chatExchange       = "chat"
	probeExchange      = "probe"
	probeReplyExchange = "probeReply"
	presenceExchange   = "presence"
)

var conn *amqp.Connection

func initAMQP() {
	var err error
	// TODO use redialer
	log.Print("Connecting to AMQP...")
	conn, err = amqp.Dial(config.AMQP)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Done.")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		chatExchange, // name of the exchange
		"direct",     // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(
		probeExchange, // name of the exchange
		"fanout",      // type
		true,          // durable
		false,         // delete when complete
		false,         // internal
		false,         // noWait
		nil,           // arguments
	)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(
		probeReplyExchange, // name of the exchange
		"direct",           // type
		true,               // durable
		false,              // delete when complete
		false,              // internal
		false,              // noWait
		nil,                // arguments
	)
	if err != nil {
		log.Fatal(err)
	}

	err = ch.ExchangeDeclare(
		presenceExchange, // name of the exchange
		"fanout",         // type
		true,             // durable
		false,            // delete when complete
		false,            // internal
		false,            // noWait
		nil,              // arguments
	)
	if err != nil {
		log.Fatal(err)
	}
}

var config struct {
	Port string `env:"PORT" default:"8080"`
	AMQP string `env:"CLOUDAMQP_URL" default:"amqp://guest:guest@localhost:5672/"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	err := envconfig.Process(&config, !*dev)
	if err != nil {
		log.Fatal(err)
	}

	initAMQP()

	fs := http.StripPrefix("/static", http.FileServer(http.Dir("static")))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) { fs.ServeHTTP(w, r) })
	http.Handle("/sockjs/", sockjs.NewHandler("/sockjs/sock", sockjs.DefaultOptions, handleSocket))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	log.Printf("Listening port %s...", config.Port)
	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		log.Fatal(err)
	}
}

// Client Message Types

type MessageType string

const (
	TypeLogin    MessageType = "login"
	TypeChat                 = "chat"
	TypePresence             = "presence"
)

type LoginMessage struct {
	Name string
}

type ChatMessage struct {
	ID     string
	From   string
	To     string
	Body   string
	Thread string
}

type PresenceMessage struct {
	Type   MessageType    `json:"type"`
	Name   string         `json:"name"`
	Status PresenceStatus `json:"status"`
}

type PresenceStatus string

const (
	StatusOnline  PresenceStatus = "online"
	StatusOffline                = "offline"
)

func logError(errp *error) {
	err := *errp
	if err != nil && err != sockjs.ErrSessionNotOpen {
		log.Print(err)
	}
}

func handleSocket(session sockjs.Session) {
	var err error
	defer logError(&err)

	var ch *amqp.Channel
	var loggedIn bool

	for {
		// Read a single message from socket.
		var sockjsMessage string
		sockjsMessage, err = session.Recv()
		if err != nil {
			return
		}
		fmt.Printf("--- received message: %s\n", sockjsMessage)
		socksjMessageBytes := []byte(sockjsMessage)

		// Determine message type.
		var message struct {
			Type MessageType
		}
		err = json.Unmarshal(socksjMessageBytes, &message)
		if err != nil {
			return
		}

		// Unmarshal JSON into correct type.
		var (
			loginMessage LoginMessage
			chatMessage  ChatMessage
		)
		var body interface{}
		switch message.Type {
		case TypeLogin:
			body = &loginMessage
		case TypeChat:
			body = &chatMessage
		default:
			err = fmt.Errorf("unknown message type: %s", message.Type)
			return
		}
		err = json.Unmarshal(socksjMessageBytes, body)
		if err != nil {
			return
		}

		// If user is not logged in yet, do not process messages other that login.
		if loggedIn && message.Type == TypeLogin {
			err = errors.New("duplicate login message")
			return
		}
		if !loggedIn && message.Type != TypeLogin {
			err = errors.New("must send login message first")
			return
		}

		switch message.Type {
		case TypeLogin:
			loggedIn = true

			ch, err = conn.Channel()
			if err != nil {
				return
			}
			defer ch.Close()

			var queue amqp.Queue
			queue, err = ch.QueueDeclare(
				"",    // name of the queue
				false, // durable
				true,  // delete when usused
				true,  // exclusive
				false, // noWait
				nil,   // arguments
			)
			if err != nil {
				return
			}
			err = ch.QueueBind(
				queue.Name,    // name of the queue
				"",            // bindingKey
				probeExchange, // sourceExchange
				false,         // noWait
				nil,           // arguments
			)
			if err != nil {
				return
			}
			err = ch.QueueBind(
				queue.Name,         // name of the queue
				loginMessage.Name,  // bindingKey
				probeReplyExchange, // sourceExchange
				false,              // noWait
				nil,                // arguments
			)
			if err != nil {
				return
			}
			err = ch.QueueBind(
				queue.Name,       // name of the queue
				"",               // bindingKey
				presenceExchange, // sourceExchange
				false,            // noWait
				nil,              // arguments
			)
			if err != nil {
				return
			}
			err = ch.QueueBind(
				queue.Name,        // name of the queue
				loginMessage.Name, // bindingKey
				chatExchange,      // sourceExchange
				false,             // noWait
				nil,               // arguments
			)
			if err != nil {
				return
			}
			err = ch.Publish(
				presenceExchange, // exchange
				"",               // routing key
				false,            // mandatory
				false,            // immediate
				amqp.Publishing{
					Headers: amqp.Table{
						"name":   loginMessage.Name,
						"status": string(StatusOnline),
					},
					DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
				},
			)
			if err != nil {
				return
			}
			defer ch.Publish(
				presenceExchange, // exchange
				"",               // routing key
				false,            // mandatory
				false,            // immediate
				amqp.Publishing{
					Headers: amqp.Table{
						"name":   loginMessage.Name,
						"status": string(StatusOffline),
					},
					DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
				},
			)
			err = ch.Publish(
				probeExchange, // exchange
				"",            // routing key
				false,         // mandatory
				false,         // immediate
				amqp.Publishing{
					Headers: amqp.Table{
						"from": loginMessage.Name,
					},
					DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
				},
			)
			if err != nil {
				return
			}
			var deliveries <-chan amqp.Delivery
			deliveries, err = ch.Consume(
				queue.Name, // name
				"",         // consumerTag,
				false,      // noAck
				false,      // exclusive
				false,      // noLocal
				false,      // noWait
				nil,        // arguments
			)
			if err != nil {
				return
			}

			go handleQueue(deliveries, session, ch, loginMessage.Name)
		case TypeChat:
			fmt.Printf("--- sending message: %s\n", sockjsMessage)
			err = ch.Publish(
				chatExchange,   // exchange
				chatMessage.To, // routing key
				false,          // mandatory
				false,          // immediate
				amqp.Publishing{
					ContentType:  "application/json",
					Body:         []byte(sockjsMessage),
					DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
				},
			)
			if err != nil {
				return
			}
		default:
			panic("unhandled message type: " + string(message.Type))
		}
	}
}

func handleQueue(deliveries <-chan amqp.Delivery, session sockjs.Session, ch *amqp.Channel, name string) {
	var err error
	defer logError(&err)

	for d := range deliveries {
		fmt.Printf("--- received delivery from exchange: %s\n", d.Exchange)
		switch d.Exchange {
		case probeExchange:
			err = ch.Publish(
				probeReplyExchange,         // exchange
				d.Headers["from"].(string), // routing key
				false, // mandatory
				false, // immediate
				amqp.Publishing{
					Headers: amqp.Table{
						"name":   name,
						"status": string(StatusOnline),
					},
					DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
				},
			)
			if err != nil {
				return
			}
		case probeReplyExchange:
			fallthrough
		case presenceExchange:
			msg := PresenceMessage{
				Type:   TypePresence,
				Name:   d.Headers["name"].(string),
				Status: PresenceStatus(d.Headers["status"].(string)),
			}
			var b []byte
			b, err = json.Marshal(&msg)
			if err != nil {
				return
			}
			err = session.Send(string(b))
			if err != nil {
				return
			}
		case chatExchange:
			err = session.Send(string(d.Body))
			if err != nil {
				return
			}
		default:
			panic("received message from unknown exchange: " + d.Exchange)
		}
		d.Ack(false)
	}
}
