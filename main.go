package main

import (
	"log"
	"net/http"
	"project/handlers"

	"github.com/gorilla/mux"
)

func main() {
	// pulsarutils.InitPulsar() // Setup Pulsar client

	router := mux.NewRouter()

	router.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Hello World ")) }).Methods("GET")

	router.HandleFunc("/create-topic/{topic}/{user}", handlers.CreateTopic).Methods("POST")
	router.HandleFunc("/send-message/{topic}/{user}", handlers.SendMessage).Methods("POST")
	router.HandleFunc("/subscribe/{topic}/{user}", handlers.Subscribe).Methods("POST")
	router.HandleFunc("/receive/{topic}/{user}", handlers.ReceiveMessages).Methods("GET")

	log.Fatal(http.ListenAndServe(":8081", router))
}
