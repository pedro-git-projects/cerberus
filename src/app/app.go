package app

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
	"github.com/pedro-git-projects/flow-sentry/operate"
	"github.com/pedro-git-projects/flow-sentry/zeebe"
)

type App struct {
	totalTests  int
	passedTests int
	failedTests int
	client      zbc.Client
	bpmnPath    string

	httpClient *http.Client
	operate    *operate.OperateService
	zeebe      *zeebe.ZeebeService
}

func New() *App {
	httpClient := &http.Client{}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	bpmnPath := filepath.Join(cwd, "workflows", "connector_test.bpmn")

	app := &App{
		bpmnPath:   bpmnPath,
		httpClient: httpClient,
	}

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

	app.initCredentials()

	app.zeebe = zeebe.NewService(client, "http://localhost:8081", httpClient)
	app.operate = operate.NewService("http://localhost:8081", httpClient)

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
