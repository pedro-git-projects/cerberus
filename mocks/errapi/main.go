package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type EnrichedData struct {
	AdditionalInfo string `json:"additional_info"`
}

func enrichData(w http.ResponseWriter, r *http.Request) {
	// Simulate API processing
	delay := time.Duration(1+rand.Intn(3)) * time.Second
	time.Sleep(delay)

	errorMessage := r.URL.Query().Get("error")

	var enrichedInfo string
	switch errorMessage {
	case "UNEXPECTED":
		enrichedInfo = "This error was unexpected. Please investigate."
	case "EXPECTED":
		enrichedInfo = "This error is known and can be handled gracefully."
	default:
		enrichedInfo = "Unknown error type."
	}

	response := EnrichedData{AdditionalInfo: enrichedInfo}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/enrich", enrichData)

	port := os.Getenv("PORT")
	if port == "" {
		port = "1337"
	}

	fmt.Printf("Starting enrichment API server on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
