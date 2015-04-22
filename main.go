package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal(err)
	}
}
