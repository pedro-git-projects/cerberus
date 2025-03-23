package zeebe

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/camunda-community-hub/zeebe-client-go/v8/pkg/zbc"
)

type ZeebeService struct {
	ZeebeClient    zbc.Client
	OperateBaseURL string
	HttpClient     *http.Client
}

func NewService(client zbc.Client, operateBaseURL string, httpClient *http.Client) *ZeebeService {
	return &ZeebeService{
		ZeebeClient:    client,
		OperateBaseURL: operateBaseURL,
		HttpClient:     httpClient,
	}
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

// StartProcess creates and starts a new process instance.
func (zs *ZeebeService) StartProcess(processID string, variables map[string]interface{}) int64 {
	ctx := context.Background()

	cmd, err := zs.ZeebeClient.NewCreateInstanceCommand().
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

// GetProcessExecution fetches and prints execution details for a process instance.
func (zs *ZeebeService) GetProcessExecution(instanceKey int64, token string) {
	url := fmt.Sprintf("%s/v1/flow-node-instances/%s", zs.OperateBaseURL, strconv.FormatInt(instanceKey, 10))
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := zs.HttpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Process Execution Details:", string(body))
}

// ParseProcessState decodes JSON data to log the state of process elements.
func (zs *ZeebeService) ParseProcessState(jsonData []byte) {
	var response struct {
		Instances []struct {
			ID              string `json:"id"`
			BpmnElementType string `json:"bpmnElementType"`
			Name            string `json:"name"`
			State           string `json:"state"`
			ErrorMessage    string `json:"errorMessage,omitempty"`
		} `json:"instances"`
	}

	err := json.Unmarshal(jsonData, &response)
	if err != nil {
		log.Fatal(err)
	}

	for _, instance := range response.Instances {
		fmt.Printf("Element: %s | State: %s\n", instance.Name, instance.State)
		if instance.ErrorMessage != "" {
			fmt.Printf("Error: %s\n", instance.ErrorMessage)
		}
	}
}
