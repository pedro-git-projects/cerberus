package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (app *App) RunTestSuites(suites []TestSuite) {
	token := app.operate.GetOperateToken()
	app.testSuites = suites

	for _, suite := range app.testSuites {
		fmt.Printf("Using unique MessageKey: %s\n", suite.MessageKey)
		fmt.Printf("\n=== 🚀 Running Test Suite for Process: %s ===\n", suite.ProcessID)

		processInstanceKey, err := app.startProcessInstance(suite, token)
		if err != nil {
			fmt.Println(err)
			app.failedTests += len(suite.TestCases)
			continue
		}

		app.runTestCases(processInstanceKey, token, suite.TestCases)
	}

	app.printSummary()
}

func (app *App) startProcessInstance(suite TestSuite, token string) (int64, error) {
	if suite.APICall != nil {
		return app.startProcessViaAPICall(suite, token)
	} else if suite.MessageName != "" {
		return app.startProcessViaMessage(suite, token)
	}
	// Direct process start when no message is provided.
	return app.zeebe.StartProcess(suite.ProcessID, suite.TestCases[0].InitialVariables), nil
}

func (app *App) startProcessViaAPICall(suite TestSuite, token string) (int64, error) {
	fmt.Printf("📨 Performing external API call: %s %s\n", suite.APICall.Method, suite.APICall.Endpoint)

	// Marshal the API payload (the JSON the external system would send)
	payloadBytes, err := json.Marshal(suite.APICall.Payload)
	if err != nil {
		return 0, fmt.Errorf("❌ Failed to marshal API payload: %v", err)
	}

	req, err := http.NewRequest(suite.APICall.Method, suite.APICall.Endpoint, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return 0, fmt.Errorf("❌ Failed to build API request: %v", err)
	}

	// Set headers from the APICall struct
	for key, value := range suite.APICall.Headers {
		req.Header.Set(key, fmt.Sprintf("%v", value))
	}

	// Perform the external API call
	resp, err := app.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("❌ Failed to perform external API request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("❌ Failed to read external API response: %v", err)
	}
	fmt.Printf("External API response: %s\n", string(body))

	// Use the new discovery function if APIMessageName is provided.
	if suite.APIMessageName != "" {
		processInstanceKey := app.discoverAPIStartedInstance(suite.ProcessID, suite.APIMessageName, token)
		if processInstanceKey == 0 {
			return 0, fmt.Errorf("❌ Could not determine process instance key after external API call")
		}
		return processInstanceKey, nil
	}

	// Fallback to discover using MessageKey
	processInstanceKey := app.discoverMessageStartedInstance(suite.ProcessID, suite.MessageKey, token)
	if processInstanceKey == 0 {
		return 0, fmt.Errorf("❌ Could not determine process instance key after external API call")
	}
	return processInstanceKey, nil
}

func (app *App) startProcessViaMessage(suite TestSuite, token string) (int64, error) {
	fmt.Printf("📨 Starting workflow via message: %s\n", suite.MessageName)
	if suite.MessageKey == "" {
		return 0, fmt.Errorf("❌ Message-based start requires 'message_key'.")
	}

	// Prepare initial variables.
	initialVars := cloneJSONMap(suite.TestCases[0].InitialVariables)
	initialVars["messageKey"] = suite.MessageKey

	variablesJSON, err := json.Marshal(initialVars)
	if err != nil {
		return 0, fmt.Errorf("❌ Failed to marshal initial variables: %v", err)
	}

	// Check for any active instance with the same correlation key.
	existingInstanceKey := app.discoverMessageStartedInstance(suite.ProcessID, suite.MessageKey, token)
	if existingInstanceKey != 0 {
		if err := app.clearExistingInstance(suite.ProcessID, suite.MessageKey, token); err != nil {
			return 0, fmt.Errorf("❌ Error clearing existing instance: %v", err)
		}
		// Poll until the previous instance is cleared.
		if !app.waitForInstanceClear(existingInstanceKey, token, 10*time.Second) {
			return 0, fmt.Errorf("❌ Timeout waiting for previous instance to clear.")
		}
	}

	// Publish the message.
	ctx := context.Background()
	cmdBuilder := app.zeebe.ZeebeClient.NewPublishMessageCommand().
		MessageName(suite.MessageName).
		CorrelationKey(suite.MessageKey)
	cmd, err := cmdBuilder.VariablesFromString(string(variablesJSON))
	if err != nil {
		return 0, fmt.Errorf("❌ Failed to apply variables to message: %v", err)
	}
	if _, err = cmd.Send(ctx); err != nil {
		return 0, fmt.Errorf("❌ Failed to publish message: %v", err)
	}

	processInstanceKey := app.discoverMessageStartedInstance(suite.ProcessID, suite.MessageKey, token)
	if processInstanceKey == 0 {
		return 0, fmt.Errorf("❌ Could not determine process instance key after message start.")
	}
	return processInstanceKey, nil
}

func (app *App) waitForInstanceClear(instanceKey int64, token string, timeout time.Duration) bool {
	cancelWaitTimeout := time.Now().Add(timeout)
	for {
		procInstance, err := app.operate.FetchProcessInstance(instanceKey, token)
		if err != nil {
			fmt.Printf("Error fetching process instance: %v\n", err)
			break
		}
		if procInstance.State != "ACTIVE" {
			return true
		}
		if time.Now().After(cancelWaitTimeout) {
			fmt.Println("❌ Timeout waiting for previous instance to clear.")
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func (app *App) runTestCases(processInstanceKey int64, token string, testCases []TestCase) {
	for _, testCase := range testCases {
		app.totalTests++
		fmt.Printf("\n=== 🧪 Validating Flow Node: %s ===\n", testCase.FlowNodeID)
		if app.validateProcessExecution(processInstanceKey, token, testCase) {
			app.passedTests++
		} else {
			app.failedTests++
		}
	}
}

func (app *App) printSummary() {
	fmt.Println("\n================= 🏁 Test Summary =================")
	fmt.Printf("Total Tests: %d | ✅ Passed: %d | ❌ Failed: %d\n", app.totalTests, app.passedTests, app.failedTests)
	if app.failedTests > 0 {
		fmt.Println("❌ Some tests failed. Please check logs for details.")
	} else {
		fmt.Println("✅ All tests passed successfully!")
	}
}

func cloneJSONMap(m JSONMap) JSONMap {
	newMap := make(JSONMap)
	for k, v := range m {
		newMap[k] = v
	}
	return newMap
}

func (app *App) discoverAPIStartedInstance(processID, expectedAPIMessageName, token string) int64 {
	payload := map[string]interface{}{
		"filter": map[string]interface{}{
			"bpmnProcessId": processID,
			"state":         "ACTIVE",
			"variables": map[string]interface{}{
				"apiMessageName": expectedAPIMessageName,
			},
		},
		"sort": []map[string]string{
			{"field": "startDate", "order": "DESC"},
		},
		"size": 1,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/process-instances/search", app.config.OperateBaseURL), strings.NewReader(string(payloadBytes)))
	if err != nil {
		log.Println("❌ Failed to build request to discover instance:", err)
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.httpClient.Do(req)
	if err != nil {
		log.Println("❌ Failed to send request to discover instance:", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Error response discovering instance: %s\n", string(body))
		return 0
	}

	var result struct {
		Items []struct {
			Key int64 `json:"key"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("❌ Failed to parse instance discovery response:", err)
		return 0
	}

	if len(result.Items) == 0 {
		log.Println("❌ No running instances found.")
		return 0
	}

	return result.Items[0].Key
}

func (app *App) discoverInstanceAfterThreshold(processID string, threshold time.Time, token string) int64 {
	payload := map[string]interface{}{
		"filter": map[string]interface{}{
			"bpmnProcessId": processID,
			"state":         "ACTIVE",
		},
		"sort": []map[string]string{
			{"field": "startDate", "order": "DESC"},
		},
		"size": 1,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/process-instances/search", app.config.OperateBaseURL), strings.NewReader(string(payloadBytes)))
	if err != nil {
		log.Println("❌ Failed to build request to discover instance:", err)
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.httpClient.Do(req)
	if err != nil {
		log.Println("❌ Failed to send request to discover instance:", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Error response discovering instance: %s\n", string(body))
		return 0
	}

	var result struct {
		Items []struct {
			Key       int64  `json:"key"`
			StartDate string `json:"startDate"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("❌ Failed to parse instance discovery response:", err)
		return 0
	}

	if len(result.Items) == 0 {
		log.Println("❌ No running instances found.")
		return 0
	}

	latest := result.Items[0]
	// Adjust this layout to match your Operate response.
	// For example, if Operate returns "2025-03-26 16:46:04", use:
	layout := "2006-01-02 15:04:05"
	startTime, err := time.Parse(layout, latest.StartDate)
	if err != nil {
		log.Printf("❌ Failed to parse startDate: %v", err)
		return 0
	}
	if startTime.Before(threshold) {
		log.Printf("❌ Latest instance started at %v is before threshold %v", startTime, threshold)
		return 0
	}
	return latest.Key
}
