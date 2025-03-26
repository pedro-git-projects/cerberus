package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pedro-git-projects/flow-sentry/utils"
)

func (app *App) validateProcessExecution(processInstanceKey int64, token string, testCase TestCase) bool {
	startTime := time.Now()

	// Wait for process instance to be available in Operate
	if !app.waitForProcessInstance(processInstanceKey, token, 10, 2*time.Second) {
		fmt.Printf("❌ Skipping test for process instance %d as it never became available.\n", processInstanceKey)
		return false
	}

	for {
		flowNodeInstanceKey, _, err := app.operate.FetchFlowNodeInstanceKey(processInstanceKey, token)
		if err != nil {
			fmt.Println("Error fetching flow node instance key:", err)

			processInstance, err := app.operate.FetchProcessInstance(processInstanceKey, token)
			if err != nil {
				fmt.Println("Error fetching process instance:", err)
				return false
			}

			if processInstance.State == "COMPLETED" {
				fmt.Println("Process instance is COMPLETED. Fetching final variable state...")

				finalVariables, err := app.operate.FetchProcessVariables(processInstanceKey, token)
				if err != nil {
					fmt.Println("Error fetching final process variables:", err)
					return false
				}

				fmt.Printf("Final Variables at process completion: %v\n", finalVariables)

				if app.validateVariables(testCase.ExpectedVariables, finalVariables) {
					fmt.Printf("✅ PASSED: Test case for flow node '%s'\n", testCase.FlowNodeID)
					return true
				}

				fmt.Printf("❌ FAILED: Test case for flow node '%s'\n", testCase.FlowNodeID)
				fmt.Printf("Expected: %v\n", testCase.ExpectedVariables)
				fmt.Printf("Actual:   %v\n", finalVariables)
				return false
			}

			continue
		}

		flowNode, err := app.operate.FetchFlowNodeInstance(flowNodeInstanceKey, token)
		if err != nil {
			fmt.Println("Error fetching flow node instance:", err)
			continue
		}

		fmt.Printf("Flow Node '%s' found. State: %s\n", flowNode.FlowNodeName, flowNode.State)

		actualVariables, err := app.operate.FetchProcessVariables(processInstanceKey, token)
		if err != nil {
			fmt.Println("Error fetching process variables:", err)
			continue
		}

		fmt.Printf("Process Variables at step '%s': %v\n", flowNode.FlowNodeName, actualVariables)

		if testCase.FlowNodeID == flowNode.FlowNodeName {
			if app.validateVariables(testCase.ExpectedVariables, actualVariables) {
				fmt.Printf("✅ PASSED: Flow Node '%s' has expected variable values.\n", flowNode.FlowNodeName)
				return true
			}

			fmt.Printf("❌ FAILED: Flow Node '%s' has unexpected variable values.\n", flowNode.FlowNodeName)
			fmt.Printf("Expected: %v\n", testCase.ExpectedVariables)
			fmt.Printf("Actual:   %v\n", actualVariables)
			return false
		}

		if time.Since(startTime) > 5*time.Minute {
			fmt.Println("Test timeout reached. Exiting.")
			break
		}

		time.Sleep(5 * time.Second)
	}

	return false
}

func (app *App) validateVariables(expected, actual map[string]interface{}) bool {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			fmt.Printf("❌ Missing expected variable: %s\n", key)
			return false
		}

		switch exp := expectedValue.(type) {
		case map[string]interface{}:
			if str, ok := actualValue.(string); ok {
				var attempt map[string]interface{}
				if json.Unmarshal([]byte(str), &attempt) == nil {
					if !reflect.DeepEqual(exp, attempt) {
						fmt.Printf("❌ Mismatch for variable '%s'.\nExpected: %v\nGot: %v\n", key, exp, attempt)
						return false
					}
					continue
				}
			}
		}

		if utils.ExtractNestedString(actualValue) != utils.ExtractNestedString(expectedValue) {
			fmt.Printf("❌ Mismatch for variable '%s'. Expected: %v, Got: %v\n", key, expectedValue, actualValue)
			return false
		}
	}
	return true
}

func (app *App) waitForProcessInstance(processInstanceKey int64, token string, maxRetries int, delay time.Duration) bool {
	for i := 0; i < maxRetries; i++ {
		_, err := app.operate.FetchProcessInstance(processInstanceKey, token)
		if err == nil {
			fmt.Printf("✅ Process instance %d is now available.\n", processInstanceKey)
			return true
		}

		fmt.Printf("⏳ Waiting for process instance %d to be available... (attempt %d/%d)\n", processInstanceKey, i+1, maxRetries)
		time.Sleep(delay)
	}

	fmt.Printf("❌ Process instance %d did not become available in time.\n", processInstanceKey)
	return false
}

func (app *App) loadTestSuites(filename string) ([]TestSuite, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var suiteConf Config
	if _, err := toml.Decode(string(data), &suiteConf); err != nil {
		return nil, err
	}

	for si, suite := range suiteConf.TestSuites {
		suiteConf.TestSuites[si].MessageKey = processUniqueMarkers(suite.MessageKey).(string)
		for ci, testCase := range suite.TestCases {
			// Process markers in any string fields inside your JSONMap.
			suiteConf.TestSuites[si].TestCases[ci].InitialVariables = processUniqueMarkers(testCase.InitialVariables).(JSONMap)
			suiteConf.TestSuites[si].TestCases[ci].ExpectedVariables = processUniqueMarkers(testCase.ExpectedVariables).(JSONMap)
		}
	}

	return suiteConf.TestSuites, nil
}

func (app *App) discoverMessageStartedInstance(processID, correlationKey, token string) int64 {
	payload := map[string]interface{}{
		"filter": map[string]interface{}{
			"bpmnProcessId": processID,
			"state":         "ACTIVE",
			// Filter by a process variable "messageKey"
			"variables": map[string]interface{}{
				"messageKey": correlationKey,
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
		fmt.Println("❌ Failed to build request to discover instance:", err)
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.httpClient.Do(req)
	if err != nil {
		fmt.Println("❌ Failed to send request to discover instance:", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("❌ Error response discovering instance: %s\n", string(body))
		return 0
	}

	var result struct {
		Items []struct {
			Key int64 `json:"key"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("❌ Failed to parse instance discovery response:", err)
		return 0
	}

	if len(result.Items) == 0 {
		fmt.Println("❌ No running instances found.")
		return 0
	}

	return result.Items[0].Key
}

func processUniqueMarkers(data interface{}) interface{} {
	switch v := data.(type) {
	case string:
		// If the string contains "#unique", replace it with a unique suffix.
		if strings.Contains(v, "#unique") {
			uniqueSuffix := utils.GenerateUniqueSuffix()
			return strings.ReplaceAll(v, "#unique", uniqueSuffix)
		}
		return v
	case JSONMap:
		for key, value := range v {
			v[key] = processUniqueMarkers(value)
		}
		return v
	case map[string]interface{}:
		for key, value := range v {
			v[key] = processUniqueMarkers(value)
		}
		return v
	case []interface{}:
		for i, value := range v {
			v[i] = processUniqueMarkers(value)
		}
		return v
	default:
		return v
	}
}

// clearExistingInstance cancels any active process instance that was started
// with the given correlation key for the specified process.
// It returns nil if no matching instance is found.
func (app *App) clearExistingInstance(processID, correlationKey, token string) error {
	// Use your discovery function to locate an active instance with the correlation key.
	instanceKey := app.discoverMessageStartedInstance(processID, correlationKey, token)
	if instanceKey == 0 {
		fmt.Printf("No active instance found for process '%s' with correlation key '%s'\n", processID, correlationKey)
		return nil
	}
	fmt.Printf("Clearing active process instance: %d\n", instanceKey)

	ctx := context.Background()
	// Use the Zeebe client's cancel command to cancel the instance.
	_, err := app.zeebe.ZeebeClient.NewCancelInstanceCommand().
		ProcessInstanceKey(instanceKey).
		Send(ctx)
	if err != nil {
		return fmt.Errorf("failed to cancel process instance %d: %w", instanceKey, err)
	}
	fmt.Printf("Successfully canceled process instance: %d\n", instanceKey)
	return nil
}
