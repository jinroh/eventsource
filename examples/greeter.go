package main

import (
	eventsource "../"
	"log"
	"net/http"
	"time"
)

func main() {
	es := eventsource.New()
	defer es.Close()
	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.Handle("/events", es)
	go func() {
		for {
			es.SendMessage([]byte("hello"), "message")
			log.Printf("Hello has been sent (consumers: %d)", es.ConsumersCount())
			time.Sleep(2 * time.Second)
		}
	}()
	log.Print("Open URL http://localhost:8080/ in your browser.")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
