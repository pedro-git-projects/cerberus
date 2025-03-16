package main

import (
	"fmt"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

var totalTests int
var passedTests int
var failedTests int

var testSuites = []struct {
	ProcessID string
	TestCases []TestCase // Steps to validate within the process instance
}{
	// Test Case: Error Handling Path
	{
		ProcessID: "Process_02q4u98",
		TestCases: []TestCase{
			{
				FlowNodeID: "Template Connector Call",
				InitialVariables: map[string]interface{}{
					"username": "nilptr",
					"token":    "very_secret_token",
					"message":  "fail - will this message reach Zeebe?",
				},
				ExpectedVariables: map[string]interface{}{
					"message":  "fail - will this message reach Zeebe?",
					"token":    "very_secret_token",
					"username": "nilptr",
				},
			},
			{
				FlowNodeID: "Deal with error",
				ExpectedVariables: map[string]interface{}{
					"caughtErr":     "EXPECTED",
					"message":       "fail - will this message reach Zeebe?",
					"token":         "very_secret_token",
					"username":      "nilptr",
					"enriched_info": "This error is known and can be handled gracefully.",
				},
			},
		},
	},

	// Test Case: Success Path
	{
		ProcessID: "Process_02q4u98",
		TestCases: []TestCase{
			{
				FlowNodeID: "Template Connector Call",
				InitialVariables: map[string]interface{}{
					"username": "nilptr",
					"token":    "very_secret_token",
					"message":  "will this message reach Zeebe?",
				},
				ExpectedVariables: map[string]interface{}{
					"message":  "will this message reach Zeebe?",
					"token":    "very_secret_token",
					"username": "nilptr",
				},
			},
			{
				FlowNodeID: "Successful Termination",
				ExpectedVariables: map[string]interface{}{
					"caughtErr": "EXPECTED",
					"message":   "will this message reach Zeebe?",
					"token":     "very_secret_token",
					"username":  "nilptr",
					"echo": map[string]interface{}{
						"myProperty": "Message received: will this message reach Zeebe?",
					},
				},
			},
		},
	},
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

	bpmnPath := "./workflows/connector_test.bpmn"
	deployWorkflow(client, bpmnPath)

	token := getOperateToken()

	for _, suite := range testSuites {
		fmt.Printf("\n=== 🚀 Running Test Suite for Process: %s ===\n", suite.ProcessID)

		// Start a single process instance for this test suite
		processInstanceKey := startProcess(client, suite.ProcessID, suite.TestCases[0].InitialVariables)

		// Validate each flow node transition within this process instance
		for _, testCase := range suite.TestCases {
			totalTests++
			fmt.Printf("\n=== 🧪 Validating Flow Node: %s ===\n", testCase.FlowNodeID)

			if validateProcessExecution(processInstanceKey, token, testCase) {
				passedTests++
			} else {
				failedTests++
			}
		}
	}

	// Print final test summary
	fmt.Println("\n================= 🏁 Test Summary =================")
	fmt.Printf("Total Tests: %d | ✅ Passed: %d | ❌ Failed: %d\n", totalTests, passedTests, failedTests)
	if failedTests > 0 {
		fmt.Println("❌ Some tests failed. Please check logs for details.")
	} else {
		fmt.Println("✅ All tests passed successfully!")
	}
}
