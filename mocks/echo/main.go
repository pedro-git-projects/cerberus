package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/entities"
	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/worker"
	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

var (
	echoChan    = make(chan string, 1)
	zeebeClient zbc.Client // Global client to be used in the HTTP handler.
)

// echoHandler processes incoming HTTP POST requests.
// It publishes a Zeebe message (with message name "Echo") to start a process instance,
// then sends the payload to echoChan for the job handler to use.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	payload := string(body)

	// Publish a message to Zeebe to start the process instance.
	// Here, we use a fixed correlation key. In a real system, you might generate one.
	correlationKey := "echo-correlation-key"
	vars := map[string]interface{}{
		"receivedPayload": payload,
	}
	varsJSON, err := json.Marshal(vars)
	if err != nil {
		log.Printf("Failed to marshal message variables: %v", err)
	} else {
		cmd, err := zeebeClient.NewPublishMessageCommand().
			MessageName("Echo").
			CorrelationKey(correlationKey).
			VariablesFromString(string(varsJSON))
		if err != nil {
			log.Printf("Failed to prepare publish message command: %v", err)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err = cmd.Send(ctx)
			if err != nil {
				log.Printf("Failed to publish message to start process instance: %v", err)
			} else {
				log.Printf("Published message to start process instance with correlation key: %s", correlationKey)
			}
		}
	}

	// Now send the payload to the channel for the job handler.
	echoChan <- payload

	log.Println("Received echo payload via HTTP:", payload)
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func main() {
	// Start the HTTP server in a separate goroutine.
	go func() {
		http.HandleFunc("/echo", echoHandler)
		log.Println("HTTP server listening on :6969")
		if err := http.ListenAndServe(":6969", nil); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Set up the Zeebe client.
	credsProvider, err := zbc.NewOAuthCredentialsProvider(&zbc.OAuthProviderConfig{
		ClientID:               "zeebe",
		ClientSecret:           "zecret",
		Audience:               "zeebe-api",
		AuthorizationServerURL: "http://localhost:18080/auth/realms/camunda-platform/protocol/openid-connect/token",
	})
	if err != nil {
		panic(err)
	}

	client, err := zbc.NewClient(&zbc.ClientConfig{
		GatewayAddress:         "localhost:26500",
		CredentialsProvider:    credsProvider,
		UsePlaintextConnection: true,
	})
	if err != nil {
		panic(err)
	}

	// Set the global zeebeClient.
	zeebeClient = client

	// Start a job worker for job type "echo".
	workerInstance := client.NewJobWorker().
		JobType("echo").
		Handler(echoJobHandler).
		Open()
	defer workerInstance.Close()

	log.Println("Worker started for 'echo'")
	// Block forever.
	select {}
}

func echoJobHandler(client worker.JobClient, job entities.Job) {
	// Wait for the HTTP /echo call to provide the payload.
	var payload string
	select {
	case payload = <-echoChan:
		// Payload received.
	case <-time.After(30 * time.Second):
		log.Printf("Timeout waiting for echo payload for job with key %d", job.Key)
		return
	}

	// Prepare variables with the "echo" field.
	vars := map[string]interface{}{
		"echo": payload,
	}
	varsJSON, err := json.Marshal(vars)
	if err != nil {
		log.Printf("Error marshalling variables for job %d: %v", job.Key, err)
		return
	}

	// Complete the Zeebe job with the variables.
	cmd, err := client.NewCompleteJobCommand().JobKey(job.Key).VariablesFromString(string(varsJSON))
	if err != nil {
		log.Printf("Failed to create complete job command for job %d: %v", job.Key, err)
		return
	}
	if _, err := cmd.Send(context.Background()); err != nil {
		log.Printf("Failed to complete job %d: %v", job.Key, err)
		return
	}

	log.Printf("Completed job %d with payload: %s", job.Key, payload)
}
