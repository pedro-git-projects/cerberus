package app

import (
	"encoding/json"
	"fmt"
)

type JSONMap map[string]interface{}

func (jm *JSONMap) UnmarshalTOML(v interface{}) error {
	switch val := v.(type) {
	case string:
		// If it's a string, assume it contains JSON.
		return json.Unmarshal([]byte(val), jm)
	case map[string]interface{}:
		*jm = val
		return nil
	default:
		return fmt.Errorf("unsupported type for JSONMap: %T", v)
	}
}

type TestCase struct {
	FlowNodeID        string  `toml:"flow_node_id" json:"flow_node_id"`
	ExpectedVariables JSONMap `toml:"expected_variables" json:"expected_variables"`
	InitialVariables  JSONMap `toml:"initial_variables" json:"initial_variables"`
}

type TestSuite struct {
	ProcessID      string     `toml:"process_id" json:"process_id"`
	WorkflowDeploy string     `toml:"workflow_deploy,omitempty" json:"workflow_deploy,omitempty"`
	MessageName    string     `toml:"message_name,omitempty" json:"message_name,omitempty"`
	MessageKey     string     `toml:"message_key,omitempty" json:"message_key,omitempty"`
	APICall        *APICall   `toml:"api_call,omitempty" json:"api_call,omitempty"`
	APIMessageName string     `toml:"api_message_name,omitempty" json:"api_message_name,omitempty"`
	TestCases      []TestCase `toml:"test_cases" json:"test_cases"`
}

type Config struct {
	TestSuites []TestSuite `toml:"test_suites" json:"test_suites`
}
