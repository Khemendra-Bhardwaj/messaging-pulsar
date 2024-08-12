package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/gorilla/mux"
)

type MessageRequest struct {
	Message string `json:"message"`
}

func userExists(userSubscriptions map[string]map[string]bool, user string) bool {
	_, ok := userSubscriptions[user]
	return ok
}

var userSubscriptions = make(map[string]map[string]bool)

var mu sync.Mutex

// CreateTopic creates a new topic.
func CreateTopic(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	topic := vars["topic"]
	user := vars["user"]

	/*Start Creating Pular Client*/
	pulsarUrl := "pulsar://localhost:6650" // for k9 : pulsar mini proxy cluster ip : 10.111.255.177
	Client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarUrl,
	})

	if err != nil {
		log.Fatalf("Error in Creating Producer Client %v", err)
	}
	log.Println("Setup Pulsar Client Done ")
	defer Client.Close()
	/*End Creating Pular Client*/

	if !userExists(userSubscriptions, user) {
		userSubscriptions[user] = make(map[string]bool)
	}

	producer, err := Client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not create producer: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("Setup Pulsar Producer Done")
	defer producer.Close()

	mu.Lock()
	if _, exists := userSubscriptions[user]; !exists {
		userSubscriptions[user] = make(map[string]bool)
	}
	userSubscriptions[user][topic] = true
	mu.Unlock()

	log.Printf("Added Topic %s for User %s ", topic, user)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Topic '%s' created successfully", topic)))
}

func SendMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	topic := vars["topic"]
	user := vars["user"]

	/*Start Creating Pular Client*/
	pulsarUrl := "pulsar://localhost:6650" // for k9 : pulsar mini proxy cluster ip : 10.111.255.177
	Client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarUrl,
	})

	if err != nil {
		log.Fatalf("Error in Creating Producer Client %v", err)
	}
	log.Println("Setup Pulsar Client Done ")
	defer Client.Close()
	/*End Creating Pular Client*/

	// TODO : check if user exist and SUBSRIBED or created that topic
	if !userExists(userSubscriptions, user) {
		http.Error(w, "User Not Found !", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Could not read request body", http.StatusBadRequest)
		return
	}

	producer, err := Client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not create producer: %v", err), http.StatusInternalServerError)
		return
	}
	defer producer.Close()

	_, err = producer.Send(context.Background(), &pulsar.ProducerMessage{
		Payload: body,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not send message: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Message sent successfully to topic  by user"))
}

func Subscribe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	user := vars["user"]
	topic := vars["topic"]
	// Check if the user exist
	if !userExists(userSubscriptions, user) {
		http.Error(w, "User Not Found ", http.StatusBadRequest)
		return
	}
	// TODO : check if the topic Exist : (main a setof current list of topics )

	// Update userSubscriptions to include the new subscription
	mu.Lock()
	if _, exists := userSubscriptions[user]; !exists {
		userSubscriptions[user] = make(map[string]bool)
	}
	userSubscriptions[user][topic] = true
	mu.Unlock()

	// Create Pulsar client
	pulsarUrl := "pulsar://localhost:6650"
	Client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarUrl,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating Pulsar client: %v", err), http.StatusInternalServerError)
		return
	}
	defer Client.Close()

	// Create a consumer for the subscription
	_, err = Client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: user,
		Type:             pulsar.Exclusive,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not create consumer: %v", err), http.StatusInternalServerError)
		return
	}

	// Respond to the client
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("User '%s' subscribed to topic '%s'", user, topic)))
}

// ReceiveMessages retrieves messages from a specified topic and subscription for a user.

func isUserSubscribedToTopic(user, topic string) bool {
	mu.Lock()
	defer mu.Unlock()
	if topics, exists := userSubscriptions[user]; exists {
		return topics[topic]
	}
	return false
}

// ReceiveMessages retrieves a message from a specified topic for a user.
func ReceiveMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	user := vars["user"]
	topic := vars["topic"]

	// Check if the user is subscribed to the topic
	if !isUserSubscribedToTopic(user, topic) {
		http.Error(w, "User is not subscribed to this topic", http.StatusForbidden)
		return
	}

	// Create Pulsar client
	pulsarUrl := "pulsar://localhost:6650"
	Client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarUrl,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating Pulsar client: %v", err), http.StatusInternalServerError)
		return
	}
	defer Client.Close()

	// Create a consumer
	consumer, err := Client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: user,
		Type:             pulsar.Exclusive,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not create consumer: %v", err), http.StatusInternalServerError)
		return
	}
	defer consumer.Close()

	// Receive a message
	msg, err := consumer.Receive(context.Background())
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not receive message: %v", err), http.StatusInternalServerError)
		return
	}

	// Acknowledge the message
	consumer.Ack(msg)

	// Prepare and send the response
	response := map[string]interface{}{
		"messageID": msg.ID(),
		"message":   string(msg.Payload()),
	}

	responseJSON, _ := json.Marshal(response)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}
