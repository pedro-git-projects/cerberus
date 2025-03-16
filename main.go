package main

import (
	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

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

	bpmnPath := "/home/pedro/dev/camunda/connector_test.bpmn"
	deployWorkflow(client, bpmnPath)

	processID := "Process_02q4u98"
	variables := map[string]interface{}{
		"username": "nilptr",
		"token":    "very_secret_token",
		"message":  "fail - will this message reach Zeebe?",
		//"message": "will this message reach Zeebe?",
	}

	startProcess(client, processID, variables)
}
