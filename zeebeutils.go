package main

import (
	"context"
	"fmt"
	"os"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

func deployWorkflow(client zbc.Client, bpmnPath string) {
	ctx := context.Background()
	file, err := os.ReadFile(bpmnPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to read BPMN file: %v", err))
	}

	deployCommand := client.NewDeployResourceCommand().AddResource(file, bpmnPath)

	_, err = deployCommand.Send(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to deploy workflow: %v", err))
	}

	fmt.Println("Workflow deployed:", bpmnPath)
}

func startProcess(client zbc.Client, processID string, variables map[string]interface{}) int64 {
	ctx := context.Background()

	cmd, err := client.NewCreateInstanceCommand().
		BPMNProcessId(processID).
		LatestVersion().
		VariablesFromMap(variables)
	if err != nil {
		panic(fmt.Sprintf("Failed to create process instance command: %v", err))
	}

	request, err := cmd.Send(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to start process instance: %v", err))
	}

	fmt.Println("Process started with instance key:", request.ProcessInstanceKey)
	return request.ProcessInstanceKey
}
