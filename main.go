package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/streadway/amqp"
	"gopkg.in/igm/sockjs-go.v2/sockjs"
)

const messagesExchange = "messages"

var conn *amqp.Connection

func initAMQP() {
	var err error
	conn, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		messagesExchange, // name of the exchange
		"direct",         // type
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

func main() {
	initAMQP()

	fs := http.StripPrefix("/static", http.FileServer(http.Dir("static")))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) { fs.ServeHTTP(w, r) })
	http.Handle("/sockjs/", sockjs.NewHandler("/sockjs/sock", sockjs.DefaultOptions, sockjsHandler))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal(err)
	}
}

type ChatMessage struct {
	From string
	To   string
	Body string
}

func sockjsHandler(session sockjs.Session) {
	var err error
	defer func() {
		if err != nil {
			log.Print(err)
			session.Close(3000, err.Error())
		}
	}()

	userName, err := session.Recv()
	if err != nil {
		return
	}

	ch, err := conn.Channel()
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
		queue.Name,       // name of the queue
		userName,         // bindingKey
		messagesExchange, // sourceExchange
		false,            // noWait
		nil,              // arguments
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

	go func() {
		var err error
		for d := range deliveries {
			fmt.Printf("--- received delivery from queue: %s\n", string(d.Body))
			err = session.Send(string(d.Body))
			if err != nil {
				break
			}
		}
	}()

	for {
		msg, err := session.Recv()
		if err != nil {
			break
		}
		fmt.Printf("--- received message from session: %s\n", msg)

		var cm ChatMessage
		err = json.Unmarshal([]byte(msg), &cm)
		if err != nil {
			break
		}

		err = ch.Publish(
			messagesExchange, // publish to an exchange
			cm.To,            // routing to 0 or more queues
			false,            // mandatory
			false,            // immediate
			amqp.Publishing{
				Headers:         amqp.Table{},
				ContentType:     "application/json",
				ContentEncoding: "",
				Body:            []byte(msg),
				DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
				Priority:        0,              // 0-9
			},
		)
		if err != nil {
			break
		}
	}
}
