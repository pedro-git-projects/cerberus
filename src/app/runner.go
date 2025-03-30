package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pedro-git-projects/flow-sentry/utils"
)

func (app *App) RunTestSuites(suites []TestSuite) {
	app.testSuites = suites

	for _, suite := range app.testSuites {
		// Process unique markers.
		suite.MessageKey = processUniqueMarkers(suite.MessageKey).(string)
		fmt.Printf("Using unique MessageKey: %s\n", suite.MessageKey)
		fmt.Printf("\n=== 🚀 Running Test Suite for Process: %s ===\n", suite.ProcessID)

		var resultVars map[string]interface{}
		var err error

		// If an API call is defined, use it to start the process.
		if suite.APICall != nil {
			// Ensure initial variables are allocated.
			if suite.TestCases[0].InitialVariables == nil {
				suite.TestCases[0].InitialVariables = make(JSONMap)
			}
			// Inject the unique message key so that the API call’s payload can be correlated.
			suite.TestCases[0].InitialVariables["messageKey"] = suite.MessageKey

			fmt.Printf("📨 Performing external API call: %s %s\n", suite.APICall.Method, suite.APICall.Endpoint)
			if err = app.performAPICall(suite.APICall); err != nil {
				fmt.Printf("❌ Failed to perform API call: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}

			// Use the subscription mechanism (via the execution listener) to retrieve the process variables.
			resultVars, err = app.subscribeToExecutionListener(5 * time.Minute)
			if err != nil {
				fmt.Printf("❌ Error waiting for execution listener notification: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}
		} else if suite.MessageName != "" {
			// Message-based start without an API call.
			if suite.MessageKey == "" {
				fmt.Println("❌ Message-based start requires 'message_key'.")
				app.failedTests += len(suite.TestCases)
				continue
			}
			if suite.TestCases[0].InitialVariables == nil {
				suite.TestCases[0].InitialVariables = make(JSONMap)
			}
			suite.TestCases[0].InitialVariables["messageKey"] = suite.MessageKey

			fmt.Printf("📨 Publishing message '%s' with correlation key '%s'\n", suite.MessageName, suite.MessageKey)
			if err := app.zeebe.PublishMessage(suite.MessageName, suite.MessageKey, suite.TestCases[0].InitialVariables); err != nil {
				fmt.Printf("❌ Failed to publish message: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}

			resultVars, err = app.subscribeToExecutionListener(5 * time.Minute)
			if err != nil {
				fmt.Printf("❌ Error waiting for execution listener notification: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}
		} else {
			// Direct process start (non message-based)
			resultVars, err = app.zeebe.WaitForProcessResult(suite.ProcessID, suite.TestCases[0].InitialVariables, 5*time.Minute)
			if err != nil {
				fmt.Printf("❌ Error waiting for process result: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}
		}

		// Validate final variables against the expected values from the last test case.
		lastTestCase := suite.TestCases[len(suite.TestCases)-1]
		if validateVariables(lastTestCase.ExpectedVariables, resultVars) {
			fmt.Printf("✅ PASSED: Process %s completed with expected variables.\n", suite.ProcessID)
			app.passedTests += len(suite.TestCases)
		} else {
			fmt.Printf("❌ FAILED: Process %s variables mismatch.\nExpected: %v\nGot: %v\n", suite.ProcessID, lastTestCase.ExpectedVariables, resultVars)
			app.failedTests += len(suite.TestCases)
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

func validateVariables(expected, actual map[string]interface{}) bool {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			fmt.Printf("❌ Missing expected variable: %s\n", key)
			return false
		}
		// For simplicity, we compare the string representations.
		if fmt.Sprintf("%v", expectedValue) != fmt.Sprintf("%v", actualValue) {
			fmt.Printf("❌ Mismatch for variable '%s'. Expected: %v, Got: %v\n", key, expectedValue, actualValue)
			return false
		}
	}
	return true
}

func processUniqueMarkers(data interface{}) interface{} {
	switch v := data.(type) {
	case string:
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

// performAPICall sends an HTTP request as defined in the APICall configuration.
func (app *App) performAPICall(apiCall *APICall) error {
	// Convert payload to a string.
	var payloadStr string
	payloadValue := interface{}(apiCall.Payload) // wrap in interface{}
	switch p := payloadValue.(type) {
	case string:
		payloadStr = p
	case map[string]interface{}:
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		payloadStr = string(b)
	default:
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		payloadStr = string(b)
	}

	req, err := http.NewRequest(apiCall.Method, apiCall.Endpoint, strings.NewReader(payloadStr))
	if err != nil {
		return err
	}

	// Convert headers to a map and set them.
	headersValue := interface{}(apiCall.Headers) // wrap in interface{}
	switch h := headersValue.(type) {
	case string:
		var headers map[string]string
		if err := json.Unmarshal([]byte(h), &headers); err != nil {
			return fmt.Errorf("failed to unmarshal headers string: %w", err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	case map[string]interface{}:
		for key, value := range h {
			req.Header.Set(key, fmt.Sprintf("%v", value))
		}
	default:
		b, err := json.Marshal(h)
		if err != nil {
			return fmt.Errorf("failed to marshal headers: %w", err)
		}
		var headers map[string]string
		if err := json.Unmarshal(b, &headers); err != nil {
			return fmt.Errorf("failed to unmarshal headers: %w", err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	resp, err := app.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
