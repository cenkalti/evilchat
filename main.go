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
	chatExchange     = "chat"
	presenceExchange = "presence"
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
	http.Handle("/sockjs/", sockjs.NewHandler("/sockjs/sock", sockjs.DefaultOptions, sockjsHandler))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	log.Printf("Listening port %s...", config.Port)
	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		log.Fatal(err)
	}
}

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
	From   string
	To     string
	Body   string
	Thread string
}

type PresenceMessage struct {
	Status string
}

type PresenceStatus string

const (
	StatusOnline  PresenceStatus = "online"
	StatusOffline                = "offline"
)

func sockjsHandler(session sockjs.Session) {
	var err error
	defer func() {
		if err != nil && err != sockjs.ErrSessionNotOpen {
			log.Print(err)
			session.Close(3000, err.Error())
		}
	}()

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

			queue, err := ch.QueueDeclare(
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
				queue.Name,        // name of the queue
				loginMessage.Name, // bindingKey
				chatExchange,      // sourceExchange
				false,             // noWait
				nil,               // arguments
			)
			if err != nil {
				return
			}
			deliveries, err := ch.Consume(
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

			go sendDeliveries(deliveries, session)
		case TypeChat:
			fmt.Printf("--- sending message: %s\n", sockjsMessage)
			err = ch.Publish(
				chatExchange,   // publish to an exchange
				chatMessage.To, // routing to 0 or more queues
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

func sendDeliveries(deliveries <-chan amqp.Delivery, session sockjs.Session) {
	for d := range deliveries {
		fmt.Printf("--- received delivery from queue: %s\n", string(d.Body))
		err := session.Send(string(d.Body))
		if err != nil {
			break
		}
	}
}
