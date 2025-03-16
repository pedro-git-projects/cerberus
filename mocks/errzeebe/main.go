package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/entities"
	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/worker"
	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

type EnrichedData struct {
	AdditionalInfo string `json:"additional_info"`
}

func fetchExternalData(errorMessage string) (EnrichedData, error) {
	url := fmt.Sprintf("http://localhost:1337/enrich?error=%s", errorMessage)
	client := http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return EnrichedData{}, fmt.Errorf("failed to call enrichment API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return EnrichedData{}, fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return EnrichedData{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var data EnrichedData
	if err := json.Unmarshal(body, &data); err != nil {
		return EnrichedData{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return data, nil
}

func main() {

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

	defer client.Close()

	client.NewJobWorker().JobType("deal_with_err").Handler(func(client worker.JobClient, job entities.Job) {
		var variables map[string]interface{}
		if err := json.Unmarshal([]byte(job.Variables), &variables); err != nil {
			fmt.Println("Error parsing job variables:", err)
			return
		}

		caughtErr, ok := variables["caughtErr"].(string)
		if !ok {
			fmt.Println("Missing 'caught_err' variable in job payload")
			return
		}

		fmt.Println("Processing error type:", caughtErr)

		enrichedData, err := fetchExternalData(caughtErr)
		if err != nil {
			fmt.Println("Failed to fetch enriched data:", err)
			return
		}

		newVars := map[string]interface{}{
			"enriched_info": enrichedData.AdditionalInfo,
		}

		cmd, err := client.NewCompleteJobCommand().JobKey(job.Key).VariablesFromMap(newVars)
		if err != nil {
			fmt.Println("Failed to create complete job command:", err)
			return
		}

		if _, err := cmd.Send(context.Background()); err != nil {
			fmt.Println("Failed to complete job:", err)
		} else {
			fmt.Println("Job successfully completed with enriched data")
		}
	}).Open()

	fmt.Println("Worker started for 'deal_with_err'")
	select {} // Keep the worker running
}
