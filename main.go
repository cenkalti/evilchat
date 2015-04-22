package main

import (
	"log"
	"net/http"
	"os"

	"gopkg.in/igm/sockjs-go.v2/sockjs"
)

func main() {
	http.Handle("/sockjs/", sockjs.NewHandler("/sockjs/sock", sockjs.DefaultOptions, echoHandler))
	http.HandleFunc("/sockjs.min.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "sockjs.min.js")
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal(err)
	}
}

func echoHandler(session sockjs.Session) {
	for {
		if msg, err := session.Recv(); err == nil {
			log.Println("received", msg)
			session.Send(msg)
			continue
		}
		break
	}
}
