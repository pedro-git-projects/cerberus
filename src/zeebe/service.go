package zeebe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

type ZeebeService struct {
	ZeebeClient zbc.Client
}

func NewService(client zbc.Client) *ZeebeService {
	return &ZeebeService{
		ZeebeClient: client,
	}
}

// WaitForProcessResult starts a process instance (or waits for an instance that was triggered via message)
// and blocks until it completes. It returns the final process variables.
func (zs *ZeebeService) WaitForProcessResult(processID string, variables map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build the create instance command and then chain WithResult() to wait for the result.
	cmd, err := zs.ZeebeClient.NewCreateInstanceCommand().
		BPMNProcessId(processID).
		LatestVersion().
		VariablesFromMap(variables)
	if err != nil {
		return nil, fmt.Errorf("failed to build create instance command: %w", err)
	}

	// Convert command to one that waits for the result.
	result, err := cmd.WithResult().Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance with result: %w", err)
	}

	var resultVars map[string]interface{}
	// Convert result.GetVariables() from string to []byte for json.Unmarshal.
	if err := json.Unmarshal([]byte(result.GetVariables()), &resultVars); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result variables: %w", err)
	}
	return resultVars, nil
}

// PublishMessage publishes a message to Zeebe to start or correlate with a process instance.
func (zs *ZeebeService) PublishMessage(messageName, correlationKey string, variables map[string]interface{}) error {
	ctx := context.Background()
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return fmt.Errorf("failed to marshal variables: %w", err)
	}
	cmd, err := zs.ZeebeClient.NewPublishMessageCommand().
		MessageName(messageName).
		CorrelationKey(correlationKey).
		VariablesFromString(string(variablesJSON))
	if err != nil {
		return fmt.Errorf("failed to build publish message command: %w", err)
	}
	_, err = cmd.Send(ctx)
	if err != nil {
		return fmt.Errorf("failed to send publish message command: %w", err)
	}
	return nil
}

// DeployWorkflow reads a BPMN file and deploys it via the Zeebe client.
func (zs *ZeebeService) DeployWorkflow(bpmnPath string) {
	ctx := context.Background()
	file, err := os.ReadFile(bpmnPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to read BPMN file: %v", err))
	}

	deployCommand := zs.ZeebeClient.NewDeployResourceCommand().AddResource(file, bpmnPath)
	_, err = deployCommand.Send(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to deploy workflow: %v", err))
	}

	fmt.Println("Workflow deployed:", bpmnPath)
}
