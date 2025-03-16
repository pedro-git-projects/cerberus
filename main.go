package main

import (
	"time"

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

	bpmnPath := "./workflows/connector_test.bpmn"
	deployWorkflow(client, bpmnPath)

	processID := "Process_02q4u98"
	variables := map[string]interface{}{
		"username": "nilptr",
		"token":    "very_secret_token",
		"message":  "fail - will this message reach Zeebe?",
		//"message": "will this message reach Zeebe?",
	}

	k := startProcess(client, processID, variables)
	token := getOperateToken()
	getProcessExecution(k, token)

	processInstanceKey := int64(2251799813748268) // Example process instance
	flowNodeId := "my_template_connector"         // Example task ID
	interval := 5 * time.Second                   // Check every 5 seconds
	timeout := 2 * time.Minute                    // Stop after 2 minutes

	monitorTaskProgress(processInstanceKey, flowNodeId, interval, timeout)
}
