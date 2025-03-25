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

func (app *App) RunTestSuites(suites []TestSuite) {
	token := app.operate.GetOperateToken()
	app.testSuites = suites

	for _, suite := range app.testSuites {

		// Process unique markers for per-field uniqueness.
		suite.MessageKey = processUniqueMarkers(suite.MessageKey).(string)
		fmt.Printf("Using unique MessageKey: %s\n", suite.MessageKey)

		fmt.Printf("\n=== 🚀 Running Test Suite for Process: %s ===\n", suite.ProcessID)

		var processInstanceKey int64
		if suite.MessageName != "" {
			fmt.Printf("📨 Starting workflow via message: %s\n", suite.MessageName)
			if suite.MessageKey == "" {
				fmt.Println("❌ Message-based start requires 'message_key'.")
				app.failedTests += len(suite.TestCases)
				continue
			}

			// Inject the unique message key into the initial variables.
			initialVars := suite.TestCases[0].InitialVariables
			initialVars["messageKey"] = suite.MessageKey

			variablesJSON, err := json.Marshal(initialVars)
			if err != nil {
				fmt.Printf("❌ Failed to marshal initial variables: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}

			// Check for any active instance with the same correlation key.
			existingInstanceKey := app.discoverMessageStartedInstance(suite.ProcessID, suite.MessageKey, token)
			if existingInstanceKey != 0 {
				// Cancel the active instance.
				if err := app.clearExistingInstance(suite.ProcessID, suite.MessageKey, token); err != nil {
					fmt.Printf("❌ Error clearing existing instance: %v\n", err)
					app.failedTests += len(suite.TestCases)
					continue
				}
				// Now poll for the cancelled instance's state.
				cancelWaitTimeout := time.Now().Add(10 * time.Second)
				cleared := false
				for {
					procInstance, err := app.operate.FetchProcessInstance(existingInstanceKey, token)
					if err != nil {
						fmt.Printf("Error fetching process instance: %v\n", err)
						break
					}
					if procInstance.State != "ACTIVE" {
						cleared = true
						break
					}
					if time.Now().After(cancelWaitTimeout) {
						fmt.Println("❌ Timeout waiting for previous instance to clear.")
						break
					}
					time.Sleep(500 * time.Millisecond)
				}
				if !cleared {
					// Skip this suite if the previous instance didn't clear.
					app.failedTests += len(suite.TestCases)
					continue
				}
			}

			// Publish the new message.
			ctx := context.Background()
			cmdBuilder := app.zeebe.ZeebeClient.NewPublishMessageCommand().
				MessageName(suite.MessageName).
				CorrelationKey(suite.MessageKey)
			cmd, err := cmdBuilder.VariablesFromString(string(variablesJSON))
			if err != nil {
				fmt.Printf("❌ Failed to apply variables to message: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}
			_, err = cmd.Send(ctx)
			if err != nil {
				fmt.Printf("❌ Failed to publish message: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}

			processInstanceKey = app.discoverMessageStartedInstance(suite.ProcessID, suite.MessageKey, token)
			if processInstanceKey == 0 {
				fmt.Println("❌ Could not determine process instance key after message start.")
				app.failedTests += len(suite.TestCases)
				continue
			}
		} else {
			// If no message is provided, start the process directly.
			processInstanceKey = app.zeebe.StartProcess(suite.ProcessID, suite.TestCases[0].InitialVariables)
		}

		// Validate each test case.
		for _, testCase := range suite.TestCases {
			app.totalTests++
			fmt.Printf("\n=== 🧪 Validating Flow Node: %s ===\n", testCase.FlowNodeID)
			if app.validateProcessExecution(processInstanceKey, token, testCase) {
				app.passedTests++
			} else {
				app.failedTests++
			}
		}
	}

	fmt.Println("\n================= 🏁 Test Summary =================")
	fmt.Printf("Total Tests: %d | ✅ Passed: %d | ❌ Failed: %d\n", app.totalTests, app.passedTests, app.failedTests)
	if app.failedTests > 0 {
		fmt.Println("❌ Some tests failed. Please check logs for details.")
	} else {
		fmt.Println("✅ All tests passed successfully!")
	}
}

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

	var conf Config
	if _, err := toml.Decode(string(data), &conf); err != nil {
		return nil, err
	}
	return conf.TestSuites, nil
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
