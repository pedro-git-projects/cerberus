package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type ProcessInstance struct {
	Key                  int64  `json:"key"`
	ProcessVersion       int    `json:"processVersion"`
	BpmnProcessId        string `json:"bpmnProcessId"`
	StartDate            string `json:"startDate"`
	State                string `json:"state"`
	Incident             bool   `json:"incident"`
	ProcessDefinitionKey int64  `json:"processDefinitionKey"`
	TenantId             string `json:"tenantId"`
}

type FlowNodeInstance struct {
	Key                  int64  `json:"key"`
	ProcessInstanceKey   int64  `json:"processInstanceKey"`
	ProcessDefinitionKey int64  `json:"processDefinitionKey"`
	FlowNodeId           string `json:"flowNodeId"`
	FlowNodeName         string `json:"flowNodeName"`
	Type                 string `json:"type"`
	State                string `json:"state"`
	Incident             bool   `json:"incident"`
}

type SequenceFlow []string

type FlowNodeSearchRequest struct {
	Filter struct {
		ProcessInstanceKey int64  `json:"processInstanceKey"`
		State              string `json:"state"`
	} `json:"filter"`
	Size int `json:"size"`
	Sort []struct {
		Field string `json:"field"`
		Order string `json:"order"`
	} `json:"sort"`
}

type FlowNodeSearchResponse struct {
	Instances []FlowNodeInstance `json:"items"`
}

// Fetches the flow node instance key for a given process instance
func fetchFlowNodeInstanceKey(processInstanceKey int64, token string) (int64, error) {
	url := "http://localhost:8081/v1/flownode-instances/search"

	requestPayload := FlowNodeSearchRequest{}
	requestPayload.Filter.ProcessInstanceKey = processInstanceKey
	requestPayload.Filter.State = "ACTIVE"
	requestPayload.Size = 1
	requestPayload.Sort = []struct {
		Field string `json:"field"`
		Order string `json:"order"`
	}{
		{"startDate", "ASC"},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return 0, fmt.Errorf("error marshalling payload: %v", err)
	}
	payload := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("HTTP request failed: %d - %s", resp.StatusCode, string(body))
	}

	var searchResponse FlowNodeSearchResponse
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response: %v", err)
	}

	err = json.Unmarshal(body, &searchResponse)
	if err != nil {
		return 0, fmt.Errorf("error parsing response JSON: %v", err)
	}

	// Ensure at least one result exists
	if len(searchResponse.Instances) == 0 {
		return 0, fmt.Errorf("no flow node instances found for processInstanceKey %d", processInstanceKey)
	}

	return searchResponse.Instances[0].Key, nil
}

func monitorTaskWithReturn(processInstanceKey int64, flowNodeId string, token string) string {
	// Step 1: Fetch the flow node instance key
	flowNodeInstanceKey, err := fetchFlowNodeInstanceKey(processInstanceKey, token)
	if err != nil {
		fmt.Println("Error fetching flow node instance key:", err)
		return ""
	}

	// Step 2: Fetch the flow node instance details
	flowNodeInstanceURL := fmt.Sprintf("http://localhost:8081/v1/flownode-instances/%d", flowNodeInstanceKey)
	var flowNode FlowNodeInstance

	err = getJSON(flowNodeInstanceURL, &flowNode, token)
	if err != nil {
		fmt.Println("Error fetching flow node instance:", err)
		return ""
	}

	fmt.Printf("Flow Node '%s' found. State: %s\n", flowNode.FlowNodeName, flowNode.State)

	// Return the current state of the flow node
	return flowNode.State
}

func monitorTask(processInstanceKey int64, flowNodeId string, token string) {
	// Step 1: Fetch the flow node instance key using process instance key
	flowNodeInstanceKey, err := fetchFlowNodeInstanceKey(processInstanceKey, token)
	if err != nil {
		fmt.Println("Error fetching flow node instance key:", err)
		return
	}

	// Step 2: Fetch the flow node instance details
	flowNodeInstanceURL := fmt.Sprintf("http://localhost:8081/v1/flownode-instances/%d", flowNodeInstanceKey)
	var flowNode FlowNodeInstance

	err = getJSON(flowNodeInstanceURL, &flowNode, token)
	if err != nil {
		fmt.Println("Error fetching flow node instance:", err)
		return
	}

	fmt.Printf("Flow Node '%s' found. State: %s\n", flowNode.FlowNodeName, flowNode.State)
}

func monitorTaskProgress(processInstanceKey int64, flowNodeId string, interval time.Duration, timeout time.Duration) {
	token := getOperateToken() // Fetch authentication token
	startTime := time.Now()
	var lastState string

	for {
		// Call monitorTaskWithReturn and get the flow node state
		currentState := monitorTaskWithReturn(processInstanceKey, flowNodeId, token)

		// Compare with last state
		if lastState != "" && currentState != lastState {
			fmt.Printf("State changed! Flow Node '%s' transitioned from '%s' to '%s'.\n", flowNodeId, lastState, currentState)
		}

		// Update last known state
		lastState = currentState

		// Stop monitoring if timeout is reached
		if time.Since(startTime) > timeout {
			fmt.Println("Monitoring timed out.")
			break
		}

		time.Sleep(interval)
	}
}
