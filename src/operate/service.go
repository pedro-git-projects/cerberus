package operate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pedro-git-projects/flow-sentry/utils"
)

type OperateService struct {
	BaseURL string
	Client  *http.Client
}

func NewService(baseURL string, httpClient *http.Client) *OperateService {
	return &OperateService{
		BaseURL: baseURL,
		Client:  httpClient,
	}
}

func (os *OperateService) FetchFlowNodeInstanceKey(processInstanceKey int64, token string) (int64, string, error) {
	url := fmt.Sprintf("%s/v1/flownode-instances/search", os.BaseURL)

	requestPayload := struct {
		Filter struct {
			ProcessInstanceKey int64  `json:"processInstanceKey"`
			State              string `json:"state"`
		} `json:"filter"`
		Size int `json:"size"`
		Sort []struct {
			Field string `json:"field"`
			Order string `json:"order"`
		} `json:"sort"`
	}{}
	requestPayload.Filter.ProcessInstanceKey = processInstanceKey
	requestPayload.Filter.State = "ACTIVE"
	requestPayload.Size = 10
	requestPayload.Sort = []struct {
		Field string `json:"field"`
		Order string `json:"order"`
	}{
		{"startDate", "ASC"},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return 0, "", fmt.Errorf("error marshalling payload: %v", err)
	}
	payload := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return 0, "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := os.Client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, "", fmt.Errorf("HTTP request failed: %d - %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Instances []struct {
			Key   int64  `json:"key"`
			State string `json:"state"`
		} `json:"items"`
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("error reading response: %v", err)
	}

	err = json.Unmarshal(body, &searchResponse)
	if err != nil {
		return 0, "", fmt.Errorf("error parsing response JSON: %v", err)
	}

	if len(searchResponse.Instances) == 0 {
		return 0, "", fmt.Errorf("no ACTIVE flow node instances found for processInstanceKey %d", processInstanceKey)
	}

	return searchResponse.Instances[0].Key, searchResponse.Instances[0].State, nil
}

// MonitorTaskWithReturn fetches the flow node instance and returns its state.
// It also demonstrates handling a COMPLETED process.
func (os *OperateService) MonitorTaskWithReturn(processInstanceKey int64, flowNodeId string, token string) string {
	flowNodeInstanceKey, _, err := os.FetchFlowNodeInstanceKey(processInstanceKey, token)
	if err != nil {
		fmt.Println("Error fetching flow node instance key:", err)

		processInstanceURL := fmt.Sprintf("%s/v1/process-instances/%d", os.BaseURL, processInstanceKey)
		var processInstance ProcessInstance

		if err := utils.GetJSON(os.Client, processInstanceURL, &processInstance, token); err != nil {
			fmt.Println("Error fetching process instance:", err)
			return ""
		}

		if processInstance.State == "COMPLETED" {
			fmt.Println("Process instance is COMPLETED. Fetching final variable state...")
			finalVariables, err := os.FetchProcessVariables(processInstanceKey, token)
			if err != nil {
				fmt.Println("Error fetching final process variables:", err)
			} else {
				fmt.Printf("Final Variables at process completion: %v\n", finalVariables)
			}
			return "COMPLETED"
		}

		if processInstance.State == "ACTIVE" {
			fmt.Println("Process is ACTIVE, but no matching flow node instance was found.")
			return ""
		}
		return ""
	}

	flowNodeInstanceURL := fmt.Sprintf("%s/v1/flownode-instances/%d", os.BaseURL, flowNodeInstanceKey)
	var flowNode FlowNodeInstance

	if err := utils.GetJSON(os.Client, flowNodeInstanceURL, &flowNode, token); err != nil {
		fmt.Println("Error fetching flow node instance:", err)
		return ""
	}

	fmt.Printf("Flow Node '%s' found. State: %s\n", flowNode.FlowNodeName, flowNode.State)

	variables, err := os.FetchProcessVariables(processInstanceKey, token)
	if err != nil {
		fmt.Println("Error fetching process variables:", err)
	} else {
		fmt.Printf("Process Variables at step '%s': %v\n", flowNode.FlowNodeName, variables)
	}

	return flowNode.State
}

// MonitorTask logs the details of a flow node instance.
func (os *OperateService) MonitorTask(processInstanceKey int64, flowNodeId string, token string) {
	flowNodeInstanceKey, _, err := os.FetchFlowNodeInstanceKey(processInstanceKey, token)
	if err != nil {
		fmt.Println("Error fetching flow node instance key:", err)
		return
	}

	flowNodeInstanceURL := fmt.Sprintf("%s/v1/flownode-instances/%d", os.BaseURL, flowNodeInstanceKey)
	var flowNode FlowNodeInstance

	if err := utils.GetJSON(os.Client, flowNodeInstanceURL, &flowNode, token); err != nil {
		fmt.Println("Error fetching flow node instance:", err)
		return
	}

	fmt.Printf("Flow Node '%s' found. State: %s\n", flowNode.FlowNodeName, flowNode.State)
}

// MonitorTaskProgress monitors changes in a task's state until completion or timeout.
func (os *OperateService) MonitorTaskProgress(processInstanceKey int64, flowNodeId string, interval, timeout time.Duration) {
	token := os.GetOperateToken() // assuming this function exists globally
	startTime := time.Now()
	var lastState string

	for {
		currentState := os.MonitorTaskWithReturn(processInstanceKey, flowNodeId, token)

		if lastState != "" && currentState != lastState {
			fmt.Printf("State changed! Flow Node '%s' transitioned from '%s' to '%s'.\n", flowNodeId, lastState, currentState)
			if currentState == "COMPLETED" {
				fmt.Println("Flow node has successfully completed execution.")
				finalVariables, err := os.FetchProcessVariables(processInstanceKey, token)
				if err != nil {
					fmt.Println("Error fetching final process variables:", err)
				} else {
					fmt.Printf("Final Variables after completion: %v\n", finalVariables)
				}
				break
			}
		}

		lastState = currentState

		if time.Since(startTime) > timeout {
			fmt.Println("Monitoring timed out.")
			break
		}
		time.Sleep(interval)
	}
}

// FetchProcessVariables retrieves process variables for a given process instance.
func (os *OperateService) FetchProcessVariables(processInstanceKey int64, token string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/variables/search", os.BaseURL)
	payload := fmt.Sprintf(`{
		"filter": {
			"processInstanceKey": %d
		},
		"size": 100
	}`, processInstanceKey)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := os.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP request failed: %d - %s", resp.StatusCode, string(body))
	}

	var variablesResponse struct {
		Items []struct {
			Name       string      `json:"name"`
			Value      interface{} `json:"value"`
			UpdateTime string      `json:"updateTime"`
		} `json:"items"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &variablesResponse); err != nil {
		return nil, err
	}

	variables := make(map[string]interface{})
	for _, v := range variablesResponse.Items {
		variables[v.Name] = v.Value
	}

	return variables, nil
}

func (os *OperateService) FetchProcessInstance(processInstanceKey int64, token string) (*ProcessInstance, error) {
	url := fmt.Sprintf("%s/v1/process-instances/%d", os.BaseURL, processInstanceKey)

	var processInstance ProcessInstance
	if err := utils.GetJSON(os.Client, url, &processInstance, token); err != nil {
		return nil, err
	}

	return &processInstance, nil
}

func (os *OperateService) FetchFlowNodeInstance(flowNodeInstanceKey int64, token string) (*FlowNodeInstance, error) {
	url := fmt.Sprintf("%s/v1/flownode-instances/%d", os.BaseURL, flowNodeInstanceKey)

	var flowNode FlowNodeInstance
	if err := utils.GetJSON(os.Client, url, &flowNode, token); err != nil {
		return nil, err
	}

	return &flowNode, nil
}

func (os *OperateService) GetOperateToken() string {
	authURL := "http://localhost:18080/auth/realms/camunda-platform/protocol/openid-connect/token"
	clientID := "zeebe"
	clientSecret := "zecret"

	data := []byte(fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=client_credentials", clientID, clientSecret))
	req, err := http.NewRequest("POST", authURL, bytes.NewBuffer(data))
	if err != nil {
		log.Fatalf("Failed to create auth request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := os.Client.Do(req)
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read auth response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("Failed to parse auth response: %v", err)
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		log.Fatalf("Authentication failed: %s", string(body))
	}

	fmt.Println("Obtained Operate API token successfully.")
	return accessToken
}
