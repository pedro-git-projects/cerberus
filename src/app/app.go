package app

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
	"github.com/pedro-git-projects/flow-sentry/operate"
	"github.com/pedro-git-projects/flow-sentry/zeebe"
)

type config struct {
	ClientID               string
	ClientSecret           string
	Audience               string
	GatewayAddress         string
	AuthorizationServerURL string
	BpmnPath               string
	OperateBaseURL         string
	SuitesPath             string
	TestSuitesFlag         string
	DeployWorkflowsFlag    string
}

type App struct {
	totalTests  int
	passedTests int
	failedTests int

	config     config
	client     zbc.Client
	testSuites []TestSuite

	httpClient *http.Client
	operate    *operate.OperateService
	zeebe      *zeebe.ZeebeService
}

func New() *App {
	httpClient := &http.Client{}
	app := &App{
		httpClient: httpClient,
	}

	// Set default config and override with flags/env vars if provided.
	app.initConfig()
	app.initFlags()
	app.initBpmnPath()
	app.initSuitesPath()
	app.initClient()

	// Initialize services using the config and client.
	app.zeebe = zeebe.NewService(app.client, app.config.OperateBaseURL, httpClient)
	app.operate = operate.NewService(app.config.OperateBaseURL, httpClient)

	return app
}

// initConfig sets basic default values.
func (app *App) initConfig() {
	app.config = config{
		ClientID:               "zeebe",
		ClientSecret:           "zecret",
		Audience:               "zeebe-api",
		GatewayAddress:         "", // will be set in initFlags
		AuthorizationServerURL: "",
		OperateBaseURL:         "",
		BpmnPath:               "",
		SuitesPath:             "",
		TestSuitesFlag:         "", // will be set in initFlags
		DeployWorkflowsFlag:    "", // will be set in initFlags
	}
}

// initFlags reads command-line flags with defaults coming from environment variables.
func (app *App) initFlags() {
	// Get environment variables or use hard-coded defaults.
	operateBaseURLDefault := os.Getenv("OPERATE_BASE_URL")
	if operateBaseURLDefault == "" {
		operateBaseURLDefault = "http://localhost:8081"
	}
	authServerURLDefault := os.Getenv("AUTHORIZATION_SERVER_URL")
	if authServerURLDefault == "" {
		authServerURLDefault = "http://localhost:18080/auth/realms/camunda-platform/protocol/openid-connect/token"
	}
	gatewayAddressDefault := os.Getenv("GATEWAY_ADDRESS")
	if gatewayAddressDefault == "" {
		gatewayAddressDefault = "localhost:26500"
	}

	// Define flags with these defaults.
	operateBaseURLFlag := flag.String("operate", operateBaseURLDefault, "Operate base URL")
	authServerURLFlag := flag.String("auth", authServerURLDefault, "Authorization server URL")
	gatewayAddressFlag := flag.String("gateway", gatewayAddressDefault, "Gateway address")
	flag.Parse()
	testSuitesFlag := flag.String("testsuites", "all", "Comma-separated list of test suite process IDs to run, or 'all' to run every suite.")
	deployWorkflowsFlag := flag.String("deployWorkflows", "none", "Workflow deployment option: 'none', 'suite', 'all', or comma-separated workflow file names.")

	app.config.OperateBaseURL = *operateBaseURLFlag
	app.config.AuthorizationServerURL = *authServerURLFlag
	app.config.GatewayAddress = *gatewayAddressFlag
	app.config.TestSuitesFlag = *testSuitesFlag
	app.config.DeployWorkflowsFlag = *deployWorkflowsFlag
}

// Execute processes workflow deployment and test suite execution based on flags.
func (app *App) Execute() {
	// Process workflow deployment.
	switch app.config.DeployWorkflowsFlag {
	case "none":
		// Do nothing.
	case "suite":
		app.DeployWorkflowsFromSuites()
	case "all":
		if err := app.DeployAllWorkflows(); err != nil {
			log.Fatalf("Failed to deploy all workflows: %v", err)
		}
	default:
		// Assume comma-separated workflow file names.
		workflowFiles := strings.Split(app.config.DeployWorkflowsFlag, ",")
		if err := app.DeployArbitraryWorkflows(workflowFiles); err != nil {
			log.Fatalf("Failed to deploy workflows: %v", err)
		}
	}

	// Process test suite selection.
	var suites []TestSuite
	var err error
	if app.config.TestSuitesFlag == "all" {
		suites, err = app.loadAllTestSuites()
	} else {
		selected := strings.Split(app.config.TestSuitesFlag, ",")
		suites, err = app.LoadSelectedTestSuites(selected)
	}
	if err != nil {
		log.Fatalf("Failed to load test suites: %v", err)
	}

	app.RunTestSuites(suites)
}

// initBpmnPath sets the BPMN file path based on the OS.
// If no BPMN path is passed, it defaults to:
//   - Linux/Mac: $HOME/.config/flow-sentry/workflows/connector_test.bpmn
//   - Windows:   %AppData%\local\flow-sentry\workflows\connector_test.bpmn
func (app *App) initBpmnPath() {
	if app.config.BpmnPath != "" {
		return
	}

	var basePath string
	if runtime.GOOS == "windows" {
		basePath = filepath.Join(os.Getenv("AppData"), "local", "flow-sentry", "workflows")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		basePath = filepath.Join(home, ".config", "flow-sentry", "workflows")
	}
	// TODO: configure BPMNs to be deployed
	// Default BPMN file name.
	app.config.BpmnPath = filepath.Join(basePath, "connector_test.bpmn")
}

// initSuitesPath sets the test suites path based on the OS.
// If none is passed, it defaults to:
//   - Linux/Mac: $HOME/.config/flow-sentry/suites
//   - Windows:   %AppData%\local\flow-sentry\suites
func (app *App) initSuitesPath() {
	if app.config.SuitesPath != "" {
		return
	}

	var basePath string
	if runtime.GOOS == "windows" {
		basePath = filepath.Join(os.Getenv("AppData"), "local", "flow-sentry", "suites")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		basePath = filepath.Join(home, ".config", "flow-sentry", "suites")
	}
	app.config.SuitesPath = basePath
}

// initClient creates the OAuth credentials provider and Zeebe client.
func (app *App) initClient() {
	credsProvider, err := zbc.NewOAuthCredentialsProvider(&zbc.OAuthProviderConfig{
		ClientID:               app.config.ClientID,
		ClientSecret:           app.config.ClientSecret,
		Audience:               app.config.Audience,
		AuthorizationServerURL: app.config.AuthorizationServerURL,
	})
	if err != nil {
		panic(err)
	}

	client, err := zbc.NewClient(&zbc.ClientConfig{
		GatewayAddress:         app.config.GatewayAddress,
		CredentialsProvider:    credsProvider,
		UsePlaintextConnection: true,
	})
	if err != nil {
		panic(err)
	}

	app.client = client
}
