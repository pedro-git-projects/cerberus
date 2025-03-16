package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

func deployWorkflow(client zbc.Client, bpmnPath string) {
	ctx := context.Background()
	file, err := os.ReadFile(bpmnPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to read BPMN file: %v", err))
	}

	deployCommand := client.NewDeployResourceCommand().AddResource(file, bpmnPath)

	_, err = deployCommand.Send(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to deploy workflow: %v", err))
	}

	fmt.Println("Workflow deployed:", bpmnPath)
}

func startProcess(client zbc.Client, processID string, variables map[string]interface{}) int64 {
	ctx := context.Background()

	cmd, err := client.NewCreateInstanceCommand().
		BPMNProcessId(processID).
		LatestVersion().
		VariablesFromMap(variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to create process instance command: %v", err))
	}

	request, err := cmd.Send(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to start process instance: %v", err))
	}

	fmt.Println("Process started with instance key:", request.ProcessInstanceKey)
	return request.ProcessInstanceKey
}

func getOperateToken() string {
	authURL := "http://localhost:18080/auth/realms/camunda-platform/protocol/openid-connect/token"
	clientID := "zeebe"
	clientSecret := "zecret"

	data := []byte(fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=client_credentials", clientID, clientSecret))

	req, err := http.NewRequest("POST", authURL, bytes.NewBuffer(data))
	if err != nil {
		log.Fatalf("Failed to create auth request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read auth response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("Failed to parse auth response: %v", err)
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		log.Fatalf("Authentication failed: %s", string(body))
	}

	fmt.Println("Obtained Operate API token successfully.")
	return accessToken
}

func getProcessExecution(instanceKey int64, token string) {
	url := fmt.Sprintf("http://localhost:8081/v1/flow-node-instances/%s", strconv.FormatInt(instanceKey, 10))

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Process Execution Details:", string(body))
}

func parseProcessState(jsonData []byte) {
	var response struct {
		Instances []struct {
			ID              string `json:"id"`
			BpmnElementType string `json:"bpmnElementType"`
			Name            string `json:"name"`
			State           string `json:"state"`
			ErrorMessage    string `json:"errorMessage,omitempty"`
		} `json:"instances"`
	}

	err := json.Unmarshal(jsonData, &response)
	if err != nil {
		log.Fatal(err)
	}

	for _, instance := range response.Instances {
		fmt.Printf("Element: %s | State: %s\n", instance.Name, instance.State)
		if instance.ErrorMessage != "" {
			fmt.Printf("Error: %s\n", instance.ErrorMessage)
		}
	}
}
