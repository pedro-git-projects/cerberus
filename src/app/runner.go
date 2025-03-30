package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pedro-git-projects/flow-sentry/utils"
)

func (app *App) RunTestSuites(suites []TestSuite) {
	app.testSuites = suites

	for _, suite := range app.testSuites {
		// Process unique markers on the test suite level.
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
			// Inject the unique message key into initial variables.
			suite.TestCases[0].InitialVariables["messageKey"] = suite.MessageKey

			fmt.Printf("📨 Performing external API call: %s %s\n", suite.APICall.Method, suite.APICall.Endpoint)
			// Pass initial variables to performAPICall so they are merged into the payload.
			if err = app.performAPICall(suite.APICall, suite.TestCases[0].InitialVariables); err != nil {
				fmt.Printf("❌ Failed to perform API call: %v\n", err)
				app.failedTests += len(suite.TestCases)
				continue
			}

			// Subscribe to the execution listener to retrieve process variables.
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
			// Direct process start (non message-based).
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

func (app *App) performAPICall(apiCall *APICall, initialVars map[string]interface{}) error {
	// Convert the payload to a string.
	var payloadStr string
	switch p := any(apiCall.Payload).(type) {
	case string:
		payloadStr = p
	case map[string]interface{}:
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal payload (map): %w", err)
		}
		payloadStr = string(b)
	default:
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal payload (default): %w", err)
		}
		payloadStr = string(b)
	}

	// Unmarshal the payload string into a map.
	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payloadMap); err != nil {
		return fmt.Errorf("failed to unmarshal payload into map: %w", err)
	}

	// Merge initialVars into payloadMap.
	for key, value := range initialVars {
		payloadMap[key] = value
	}

	// Build a context map for substitution.
	contextMap := make(map[string]string)
	if msgKey, exists := initialVars["messageKey"]; exists {
		if keyStr, ok := msgKey.(string); ok {
			contextMap["echo_correlation_key"] = keyStr
		}
	}

	// Substitute unique markers and placeholders in payloadMap.
	substituted := substituteUniqueAndVariablesWithContext(payloadMap, contextMap)
	finalPayloadMap, ok := substituted.(map[string]interface{})
	if !ok {
		return fmt.Errorf("substitution did not return a map[string]interface{}")
	}
	payloadMap = finalPayloadMap

	// Marshal the final payload.
	mergedPayload, err := json.Marshal(payloadMap)
	if err != nil {
		return fmt.Errorf("failed to marshal merged payload: %w", err)
	}
	log.Printf("Merged payload: %s", string(mergedPayload))

	req, err := http.NewRequest(apiCall.Method, apiCall.Endpoint, strings.NewReader(string(mergedPayload)))
	if err != nil {
		return err
	}

	// Process headers.
	switch h := any(apiCall.Headers).(type) {
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

// substituteUniqueAndVariables traverses a JSONMap (map[string]interface{})
// and substitutes any strings containing "#unique" with a generated unique value,
// and then replaces any placeholders of the form "${var}" with the value from the context.
func substituteUniqueAndVariables(m map[string]interface{}) map[string]interface{} {
	// context to store generated variables (e.g. for keys with "#unique")
	contextMap := make(map[string]string)

	// First pass: generate unique values for any key whose value contains "#unique".
	var generateUnique func(data interface{}) interface{}
	generateUnique = func(data interface{}) interface{} {
		switch v := data.(type) {
		case string:
			// If the string contains "#unique", replace it.
			if strings.Contains(v, "#unique") {
				// If the entire value is exactly "#unique" or contains a pattern, generate a unique suffix.
				// Here we simply replace "#unique" with a generated suffix.
				unique := utils.GenerateUniqueSuffix()
				return strings.ReplaceAll(v, "#unique", unique)
			}
			return v
		case map[string]interface{}:
			for key, value := range v {
				// If the value is a string and contains "#unique", generate a unique value and store it in the context.
				if s, ok := value.(string); ok && strings.Contains(s, "#unique") {
					uniqueVal := strings.ReplaceAll(s, "#unique", utils.GenerateUniqueSuffix())
					v[key] = uniqueVal
					contextMap[key] = uniqueVal
				} else {
					v[key] = generateUnique(value)
				}
			}
			return v
		case []interface{}:
			for i, item := range v {
				v[i] = generateUnique(item)
			}
			return v
		default:
			return data
		}
	}

	// Second pass: substitute placeholders in strings.
	placeholderRegexp := regexp.MustCompile(`\$\{([^}]+)\}`)
	var substitutePlaceholders func(data interface{}) interface{}
	substitutePlaceholders = func(data interface{}) interface{} {
		switch v := data.(type) {
		case string:
			// Replace every occurrence of ${key} with contextMap[key] if available.
			return placeholderRegexp.ReplaceAllStringFunc(v, func(match string) string {
				// match is like "${key}", extract key:
				key := placeholderRegexp.FindStringSubmatch(match)[1]
				if val, ok := contextMap[key]; ok {
					return val
				}
				// If not found in context, leave it unchanged.
				return match
			})
		case map[string]interface{}:
			for key, value := range v {
				v[key] = substitutePlaceholders(value)
			}
			return v
		case []interface{}:
			for i, item := range v {
				v[i] = substitutePlaceholders(item)
			}
			return v
		default:
			return data
		}
	}

	generateUnique(m)
	substitutePlaceholders(m)
	return m
}

// substituteUniqueAndVariablesWithContext recursively walks data (which may be a map or slice)
// and replaces any string containing "#unique" with a generated unique value. It also replaces
// placeholders of the form "${var}" with the value from contextMap.
func substituteUniqueAndVariablesWithContext(data interface{}, contextMap map[string]string) interface{} {
	placeholderRegexp := regexp.MustCompile(`\$\{([^}]+)\}`)
	switch v := data.(type) {
	case string:
		// First, if the string contains "#unique", generate a unique value.
		if strings.Contains(v, "#unique") {
			unique := utils.GenerateUniqueSuffix()
			v = strings.ReplaceAll(v, "#unique", unique)
			// Optionally, if you want to capture this in context, you can do so here.
		}
		// Replace placeholders: e.g. "${echo_correlation_key}".
		return placeholderRegexp.ReplaceAllStringFunc(v, func(match string) string {
			key := placeholderRegexp.FindStringSubmatch(match)[1]
			if val, ok := contextMap[key]; ok {
				return val
			}
			return match
		})
	case map[string]interface{}:
		for key, val := range v {
			v[key] = substituteUniqueAndVariablesWithContext(val, contextMap)
		}
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = substituteUniqueAndVariablesWithContext(item, contextMap)
		}
		return v
	default:
		return v
	}
}
