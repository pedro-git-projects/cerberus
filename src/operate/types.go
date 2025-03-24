package operate

type ProcessInstance struct {
	Key                  int64  `json:"key"`
	ProcessVersion       int    `json:"processVersion"`
	BpmnProcessId        string `json:"bpmnProcessId"`
	StartDate            string `json:"startDate"`
	State                string `json:"state"`
	Incident             bool   `json:"incident"`
	ProcessDefinitionKey int64  `json:"processDefinitionKey"`
	TenantId             string `json:"tenantId"`
}

type FlowNodeInstance struct {
	Key                  int64  `json:"key"`
	ProcessInstanceKey   int64  `json:"processInstanceKey"`
	ProcessDefinitionKey int64  `json:"processDefinitionKey"`
	FlowNodeId           string `json:"flowNodeId"`
	FlowNodeName         string `json:"flowNodeName"`
	Type                 string `json:"type"`
	State                string `json:"state"`
	Incident             bool   `json:"incident"`
}

type SequenceFlow []string

type FlowNodeSearchRequest struct {
	Filter struct {
		ProcessInstanceKey int64  `json:"processInstanceKey"`
		State              string `json:"state"`
	} `json:"filter"`
	Size int `json:"size"`
	Sort []struct {
		Field string `json:"field"`
		Order string `json:"order"`
	} `json:"sort"`
}

type FlowNodeSearchResponse struct {
	Instances []FlowNodeInstance `json:"items"`
}
