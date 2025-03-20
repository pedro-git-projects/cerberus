package main

import (
	"fmt"
	"log"

	"io/ioutil"

	"github.com/BurntSushi/toml"
	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

type TestCase struct {
	FlowNodeID        string                 `toml:"flow_node_id"`
	ExpectedVariables map[string]interface{} `toml:"expected_variables"`
	InitialVariables  map[string]interface{} `toml:"initial_variables"`
}

type TestSuite struct {
	ProcessID string     `toml:"process_id"`
	TestCases []TestCase `toml:"test_cases"`
}

type Config struct {
	TestSuites []TestSuite `toml:"test_suites"`
}

func LoadTestSuites(filename string) ([]TestSuite, error) {
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

type App struct {
	totalTests  int
	passedTests int
	failedTests int
	client      zbc.Client
	bpmnPath    string
}

func NewApp() *App {
	app := &App{}
	app.initCredentials()
	app.bpmnPath = "./workflows/connector_test.bpmn"
	return app
}

func (app *App) initCredentials() {
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

	app.client = client
}

func (app *App) deployBpmn() {
	deployWorkflow(app.client, app.bpmnPath)
}

func (app *App) RunTestSuites(suites []TestSuite) {
	token := getOperateToken()

	for _, suite := range suites {
		fmt.Printf("\n=== 🚀 Running Test Suite for Process: %s ===\n", suite.ProcessID)

		processInstanceKey := startProcess(app.client, suite.ProcessID, suite.TestCases[0].InitialVariables)

		for _, testCase := range suite.TestCases {
			app.totalTests++
			fmt.Printf("\n=== 🧪 Validating Flow Node: %s ===\n", testCase.FlowNodeID)

			if validateProcessExecution(processInstanceKey, token, testCase) {
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

func main() {
	app := NewApp()
	app.deployBpmn()

	suites, err := LoadTestSuites("testsuites.toml")
	if err != nil {
		log.Fatalf("Error loading test suites: %v", err)
	}

	app.RunTestSuites(suites)
}
