package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/redialer/amqpredialer"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
	"gopkg.in/igm/sockjs-go.v2/sockjs"
)

var (
	rabbit *amqpredialer.AMQPRedialer
	db     *sql.DB
)

var config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	AMQP        string `envconfig:"CLOUDAMQP_URL" default:"amqp://guest:guest@localhost:5672/"`
	PostgresURL string `envconfig:"DATABASE_URL" default:"postgres://localhost/?sslmode=disable&dbname=evilchat"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal(err)
	}

	db, err = sql.Open("postgres", config.PostgresURL)
	if err != nil {
		log.Fatal(err)
	}

	rabbit, err = amqpredialer.New(config.AMQP)
	if err != nil {
		log.Fatal(err)
	}

	go rabbit.Run()

	conn := <-rabbit.Conn()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	if err = exchangeDeclare(ch, chatExchange, "direct"); err != nil {
		log.Fatal(err)
	}
	if err = exchangeDeclare(ch, probeExchange, "direct"); err != nil {
		log.Fatal(err)
	}
	if err = exchangeDeclare(ch, probeReplyExchange, "direct"); err != nil {
		log.Fatal(err)
	}
	if err = exchangeDeclare(ch, presenceExchange, "direct"); err != nil {
		log.Fatal(err)
	}
	err = ch.Close()
	if err != nil {
		log.Fatal(err)
	}

	fs := http.StripPrefix("/static", http.FileServer(http.Dir("static")))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) { fs.ServeHTTP(w, r) })
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "favicon.ico") })
	http.Handle("/sockjs/", sockjsHandlerWithRequest("/sockjs/sock", sockjs.DefaultOptions, handleSocket))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	log.Printf("Listening port %s...", config.Port)
	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		log.Fatal(err)
	}
}

// sockjsHandlerWithRequest is a wrapper around the sockjs.Handler that
// includes a *http.Request context.
func sockjsHandlerWithRequest(prefix string, opts sockjs.Options, handleFunc func(sockjs.Session, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sockjs.NewHandler(prefix, opts, func(session sockjs.Session) {
			handleFunc(session, r)
		}).ServeHTTP(w, r)
	})
}

// AMQP functions with some defaults.
func exchangeDeclare(ch *amqp.Channel, name, kind string) error {
	return ch.ExchangeDeclare(
		name,  // name of the exchange
		kind,  // type
		true,  // durable
		false, // delete when complete
		false, // internal
		false, // noWait
		nil,   // arguments
	)
}
func queueBind(ch *amqp.Channel, queue, routingKey, exchange string) error {
	return ch.QueueBind(
		queue,      // name of the queue
		routingKey, // bindingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	)
}
func publish(ch *amqp.Channel, exchange, routingKey string, body []byte, headers amqp.Table) error {
	p := amqp.Publishing{
		Headers:      headers,
		DeliveryMode: amqp.Transient, // 1=non-persistent, 2=persistent
	}
	if body != nil {
		p.ContentType = "application/json"
		p.Body = body
	}
	return ch.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		p,
	)
}

type MessageType string
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
const (
	TypeLogin    MessageType = "login"
	TypeChat                 = "chat"
	TypePresence             = "presence"
)
const (
	chatExchange       = "chat"
	probeExchange      = "probe"
	probeReplyExchange = "probeReply"
	presenceExchange   = "presence"
)

var logger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)

func logError(errp *error) {
	err := *errp
	if err != nil && err != sockjs.ErrSessionNotOpen {
		logger.Output(2, err.Error())
	}
}

func handleSocket(session sockjs.Session, r *http.Request) {
	var err error
	defer logError(&err)

	var ch *amqp.Channel
	var loggedIn bool

	team := strings.SplitN(strings.SplitN(r.Host, ".", 2)[0], ":", 2)[0] // sub-domain part

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

			var conn *amqp.Connection
			select {
			case conn = <-rabbit.Conn():
			case <-time.After(time.Second):
				err = errors.New("cannot connect to backend")
				return
			}

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

			if err = queueBind(ch, queue.Name, team, probeExchange); err != nil {
				return
			}
			if err = queueBind(ch, queue.Name, team+"."+loginMessage.Name, probeReplyExchange); err != nil {
				return
			}
			if err = queueBind(ch, queue.Name, team, presenceExchange); err != nil {
				return
			}
			if err = queueBind(ch, queue.Name, team+"."+loginMessage.Name, chatExchange); err != nil {
				return
			}
			err = publish(ch, presenceExchange, team, nil, amqp.Table{
				"name":   loginMessage.Name,
				"status": string(StatusOnline),
			})
			if err != nil {
				return
			}
			defer publish(ch, presenceExchange, team, nil, amqp.Table{
				"name":   loginMessage.Name,
				"status": string(StatusOffline),
			})
			err = publish(ch, probeExchange, team, nil, amqp.Table{
				"from": loginMessage.Name,
			})
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

			go handleQueue(deliveries, session, ch, team, loginMessage.Name)
		case TypeChat:
			fmt.Printf("--- sending message: %s\n", sockjsMessage)
			err = publish(ch, chatExchange, team+"."+chatMessage.To, []byte(sockjsMessage), nil)
			if err != nil {
				return
			}
		default:
			panic("unhandled message type: " + string(message.Type))
		}
	}
}

func handleQueue(deliveries <-chan amqp.Delivery, session sockjs.Session, ch *amqp.Channel, team, name string) {
	var err error
	defer logError(&err)

	for d := range deliveries {
		fmt.Printf("--- received delivery from exchange: %s\n", d.Exchange)
		switch d.Exchange {
		case probeExchange:
			err = publish(ch, probeReplyExchange, team+"."+d.Headers["from"].(string), nil, amqp.Table{
				"name":   name,
				"status": string(StatusOnline),
			})
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
