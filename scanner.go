package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v2/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/go-github/v66/github"
	"github.com/joho/godotenv"
)

var (
	ghToken    string
	amqpUri    string
	ghClient   *github.Client
	publisher  *amqp.Publisher
	executions chan (struct{})
	cache      ResultCache
)

func processMessages(messages <-chan *message.Message) {
	for msg := range messages {
		var scanMessage ScanMessage
		jsonError := json.Unmarshal([]byte(msg.Payload), &scanMessage)
		if jsonError != nil {
			log.Printf("Error unmarshalling this object into a scan message: '%v'", string(msg.Payload))
			msg.Ack() // In Prod would probably move to some other queue to look at later
		}

		if scanMessage.ScanStatus != ScanDone {
			go ScanRepo(executions, publisher, ghClient, scanMessage.RepoName, scanMessage.RepoAuthor, scanMessage.LastPage+1)
			fmt.Printf("Scanning page %d on repo %s\n", scanMessage.LastPage+1, scanMessage.RepoName)
		}

		msg.Ack()
	}
}

func processResults(messages <-chan *message.Message) {
	for msg := range messages {
		var scanResult ScanResult
		if jsonError := json.Unmarshal([]byte(msg.Payload), &scanResult); jsonError != nil {
			log.Printf("Error unmarshalling this object into a scan message: '%v'", string(msg.Payload))
			msg.Ack() // In Prod would probably move to some other queue to look at later
		}

		repoKey := fmt.Sprintf("%s/%s", scanResult.RepoAuthor, scanResult.RepoName)
		cache.AddResult(repoKey, scanResult.Results)
	}
}

func loadDotenv() error {
	err := godotenv.Load()
	if err != nil {
		return err
	}

	var envExists bool
	amqpUri, envExists = os.LookupEnv("RABBITMQ_SERVER_URL")
	if !envExists {
		return fmt.Errorf("environment variable \"RABBITMQ_SERVER_URL\" does not exist")
	}

	ghToken, envExists = os.LookupEnv("GITHUB_TOKEN")
	if !envExists {
		return fmt.Errorf("environment variable \"GITHUB_TOKEN\" does not exist")
	}

	return nil
}

func getHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Running")
}

func getScan(w http.ResponseWriter, r *http.Request) {
	author := r.URL.Query().Get("author")
	repoUrl := r.URL.Query().Get("repo")

	scanMessage := NewScanMessage(author, repoUrl)
	jsonMessage, err := json.Marshal(scanMessage)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), jsonMessage)
	publisher.Publish("scans.topic", msg)

	fmt.Fprintln(w, "Done")
}

func getRepo(w http.ResponseWriter, r *http.Request) {
	author := r.URL.Query().Get("author")
	repoName := r.URL.Query().Get("repo")

	repoKey := fmt.Sprintf("%s/%s", author, repoName)
	results, found := cache.GetResults(repoKey)
	if !found {
		http.Error(w, fmt.Sprintf("No results for repo '%s'", repoKey), 400)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	content, err := json.Marshal(results)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error marshalling results: %v", err), 500)
		return
	}

	w.Write(content)
}

func main() {
	if err := loadDotenv(); err != nil {
		log.Fatalf("Error loading env vars: %v", err)
	}

	amqpConfig := amqp.NewDurableQueueConfig(amqpUri)
	subscriber, err := amqp.NewSubscriber(amqpConfig, watermill.NewStdLogger(false, false))
	if err != nil {
		log.Fatalf("Error creating AMQP subscriber: %v", err)
	}

	publisher, err = amqp.NewPublisher(amqpConfig, watermill.NewStdLogger(false, false))
	if err != nil {
		log.Fatalf("Error creating AMQP publisher: %v", err)
	}

	scanMessages, err := subscriber.Subscribe(context.Background(), "scans.topic")
	if err != nil {
		log.Fatalf("Error subscribing to scans.topic: %v", err)
	}

	resultMessages, err := subscriber.Subscribe(context.Background(), "results.topic")
	if err != nil {
		log.Fatalf("Error subscribing to scans.topic: %v", err)
	}

	ghClient = github.NewClient(nil).WithAuthToken(ghToken)
	cache = NewResultCache()

	// Our executions channel allows two goroutines per CPU core
	executions = make(chan struct{}, runtime.NumCPU()*2)

	go processMessages(scanMessages)
	go processResults(resultMessages)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", getHealth)
	mux.HandleFunc("/scan", getScan)
	mux.HandleFunc("/repo", getRepo)

	err = http.ListenAndServe(":3000", mux)
	log.Fatal(err)
}
