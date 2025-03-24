package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pedro-git-projects/flow-sentry/utils"
)

func (app *App) RunTestSuites() {
	token := app.operate.GetOperateToken()
	suites, err := app.loadTestSuites(app.config.SuitesPath)
	if err != nil {
		log.Fatalf("Failed to load test suites %v", err)
	}
	app.testSuites = suites

	for _, suite := range app.testSuites {
		fmt.Printf("\n=== 🚀 Running Test Suite for Process: %s ===\n", suite.ProcessID)

		processInstanceKey := app.zeebe.StartProcess(suite.ProcessID, suite.TestCases[0].InitialVariables)

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
