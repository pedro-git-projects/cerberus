package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

func validateProcessExecution(processInstanceKey int64, token string, testCase TestCase) bool {
	startTime := time.Now()

	// Wait for process instance to be indexed in Operate
	if !waitForProcessInstance(processInstanceKey, token, 10, 2*time.Second) {
		fmt.Printf("❌ Skipping test for process instance %d as it never became available.\n", processInstanceKey)
		return false
	}

	for {
		// Fetch active flow node instance key
		flowNodeInstanceKey, _, err := fetchFlowNodeInstanceKey(processInstanceKey, token)
		if err != nil {
			fmt.Println("Error fetching flow node instance key:", err)

			// Check if process is completed
			processInstanceURL := fmt.Sprintf("http://localhost:8081/v1/process-instances/%d", processInstanceKey)
			var processInstance ProcessInstance

			err = getJSON(processInstanceURL, &processInstance, token)
			if err != nil {
				fmt.Println("Error fetching process instance:", err)
				return false
			}

			if processInstance.State == "COMPLETED" {
				fmt.Println("Process instance is COMPLETED. Fetching final variable state...")

				finalVariables, err := fetchProcessVariables(processInstanceKey, token)
				if err != nil {
					fmt.Println("Error fetching final process variables:", err)
					return false
				}

				fmt.Printf("Final Variables at process completion: %v\n", finalVariables)

				// Validate variables at process completion
				if validateVariables(testCase.ExpectedVariables, finalVariables) {
					fmt.Printf("✅ PASSED: Test case for flow node '%s'\n", testCase.FlowNodeID)
					return true
				} else {
					fmt.Printf("❌ FAILED: Test case for flow node '%s'\n", testCase.FlowNodeID)
					fmt.Printf("Expected: %v\n", testCase.ExpectedVariables)
					fmt.Printf("Actual:   %v\n", finalVariables)
					return false
				}
			}
			continue
		}

		// Fetch flow node details
		flowNodeInstanceURL := fmt.Sprintf("http://localhost:8081/v1/flownode-instances/%d", flowNodeInstanceKey)
		var flowNode FlowNodeInstance

		err = getJSON(flowNodeInstanceURL, &flowNode, token)
		if err != nil {
			fmt.Println("Error fetching flow node instance:", err)
			continue
		}

		fmt.Printf("Flow Node '%s' found. State: %s\n", flowNode.FlowNodeName, flowNode.State)

		// Fetch actual process variables
		actualVariables, err := fetchProcessVariables(processInstanceKey, token)
		if err != nil {
			fmt.Println("Error fetching process variables:", err)
			continue
		}

		fmt.Printf("Process Variables at step '%s': %v\n", flowNode.FlowNodeName, actualVariables)

		// Validate against expected variables
		if testCase.FlowNodeID == flowNode.FlowNodeName {
			if validateVariables(testCase.ExpectedVariables, actualVariables) {
				fmt.Printf("✅ PASSED: Flow Node '%s' has expected variable values.\n", flowNode.FlowNodeName)
				return true
			} else {
				fmt.Printf("❌ FAILED: Flow Node '%s' has unexpected variable values.\n", flowNode.FlowNodeName)
				fmt.Printf("Expected: %v\n", testCase.ExpectedVariables)
				fmt.Printf("Actual:   %v\n", actualVariables)
				return false
			}
		}

		// Stop monitoring after 5 minutes
		if time.Since(startTime) > 5*time.Minute {
			fmt.Println("Test timeout reached. Exiting.")
			break
		}

		time.Sleep(5 * time.Second)
	}

	return false
}

func validateVariables(expected, actual map[string]interface{}) bool {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			fmt.Printf("❌ Missing expected variable: %s\n", key)
			return false
		}

		// <-- If expected is a map, but actual is a JSON string, try to parse it.
		switch exp := expectedValue.(type) {
		case map[string]interface{}:
			// The test expects a map. Check if actualValue is a string containing JSON
			if str, ok := actualValue.(string); ok {
				var attempt map[string]interface{}
				if json.Unmarshal([]byte(str), &attempt) == nil {
					// now compare attempt vs exp
					if !reflect.DeepEqual(exp, attempt) {
						fmt.Printf("❌ Mismatch for variable '%s'.\nExpected: %v\nGot: %v\n", key, exp, attempt)
						return false
					}
					// match is good, continue
					continue
				}
			}
		}

		// If neither is a map nor a JSON string, fall back to normal comparison...
		if extractNestedString(actualValue) != extractNestedString(expectedValue) {
			fmt.Printf("❌ Mismatch for variable '%s'. Expected: %v, Got: %v\n",
				key, expectedValue, actualValue)
			return false
		}
	}
	return true
}

func waitForProcessInstance(processInstanceKey int64, token string, maxRetries int, delay time.Duration) bool {
	for i := 0; i < maxRetries; i++ {
		// Fetch process instance
		processInstanceURL := fmt.Sprintf("http://localhost:8081/v1/process-instances/%d", processInstanceKey)
		var processInstance ProcessInstance

		err := getJSON(processInstanceURL, &processInstance, token)
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
