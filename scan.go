package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v2/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/go-github/v66/github"
)

const (
	commitsPerPage = 1000
)

type CommitErrorResult struct {
	CommitID    string `json:"commit"`
	FileName    string `json:"filename"`
	Line        int    `json:"line"`
	SecretFound string `json:"secret"`
}

type ScanResult struct {
	RepoName   string              `json:"repo_name"`
	RepoAuthor string              `json:"repo_author"`
	IsDone     bool                `json:"is_done"`
	Error      error               `json:"error"`
	Results    []CommitErrorResult `json:"results"`
}

// Could've been simplified a bit with a context...
func ScanRepo(executions chan (struct{}), resultPublisher *amqp.Publisher, ghClient *github.Client, repo string, author string, page int) {
	executions <- struct{}{}

	scanResult := ScanResult{
		RepoName:   repo,
		RepoAuthor: author,
		IsDone:     false,
		Error:      nil,
		Results:    make([]CommitErrorResult, 0),
	}

	commits, _, err := ghClient.Repositories.ListComments(context.Background(), author, repo,
		&github.ListOptions{
			Page:    page,
			PerPage: commitsPerPage,
		})
	if err != nil {
		// TODO: Probably best to post to some other channel to look at later
		log.Fatalf("Error = %v", err)
		scanResult.IsDone = true
	}

	log.Printf("Done scannign page %d", len(commits))

	if len(commits) == 0 {
		scanResult.IsDone = true
	}

	// Post result
	resultMsg, err := json.Marshal(scanResult)
	if err != nil {
		// TODO: Probably best to post to some other channel to look at later
		log.Printf("Error marshalling JSON: %v", err)
		scanResult.Error = err
		scanResult.IsDone = true
	}
	msg := message.NewMessage(watermill.NewUUID(), resultMsg)
	publisher.Publish("results.topic", msg)

	// Immediately post to scan the next page if needed
	if !scanResult.IsDone {
		nextPageMsg := NewScanMessage(author, repo)
		nextPageMsg.LastPage++
		nextPageMsg.ScanStatus = Scanning
		jsonMessage, err := json.Marshal(nextPageMsg)
		if err != nil {
			// TODO: Probably best to post to some other channel to look at later
			log.Printf("Error marshalling JSON: %v", err)
			scanResult.Error = err
			scanResult.IsDone = true
		}

		msg = message.NewMessage(watermill.NewUUID(), jsonMessage)
		publisher.Publish("scans.topic", msg)
	}

	<-executions
}
