package app

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// TestJSONMapParsing checks that JSONMap correctly handles both a JSON string and a table.
func TestJSONMapParsing(t *testing.T) {
	// Case 1: When a JSON string is provided.
	var jm JSONMap
	jsonInput := `{"username": "nilptr", "token": "very_secret_token", "message": "hello"}`
	if err := jm.UnmarshalTOML(jsonInput); err != nil {
		t.Fatalf("UnmarshalTOML failed for JSON string: %v", err)
	}
	// Verify expected values.
	if jm["username"] != "nilptr" || jm["token"] != "very_secret_token" || jm["message"] != "hello" {
		t.Errorf("Unexpected JSONMap contents from JSON string: %v", jm)
	}

	// Case 2: When a TOML table (i.e. map) is provided.
	inputTable := map[string]interface{}{
		"username": "nilptr",
		"token":    "very_secret_token",
		"message":  "world",
	}
	var jm2 JSONMap
	if err := jm2.UnmarshalTOML(inputTable); err != nil {
		t.Fatalf("UnmarshalTOML failed for table: %v", err)
	}
	if jm2["username"] != "nilptr" || jm2["token"] != "very_secret_token" || jm2["message"] != "world" {
		t.Errorf("Unexpected JSONMap contents from table: %v", jm2)
	}
}

// TestUniqueSubstitutions verifies that any occurrence of "#unique" in a string is replaced.
func TestUniqueSubstitutions(t *testing.T) {
	original := "test-key#unique"
	substituted := processUniqueMarkers(original)
	result, ok := substituted.(string)
	if !ok {
		t.Fatalf("Expected a string result from substitution, got %T", substituted)
	}
	// Check that "#unique" no longer appears.
	if strings.Contains(result, "#unique") {
		t.Errorf("Expected '#unique' to be replaced; got: %s", result)
	}
	// Check that the rest of the string remains.
	if !strings.HasPrefix(result, "test-key") {
		t.Errorf("Expected result to start with 'test-key'; got: %s", result)
	}

	// Also, if no "#unique" is present, the string should remain unchanged.
	original2 := "normal-string"
	substituted2 := processUniqueMarkers(original2)
	if substituted2 != original2 {
		t.Errorf("Expected %s, got %s", original2, substituted2)
	}
}

// TestConfigParsing decodes a sample TOML configuration and verifies that
// both JSONMap decoding and "#unique" substitutions occur.
func TestConfigParsing(t *testing.T) {
	// This sample uses triple-double-quoted strings to force the TOML decoder to treat them as strings.
	tomlData := `
[[test_suites]]
process_id = "Process_02q4u98"
message_name = "Message_0um7vvb#unique"
message_key = "test-key#unique"

[[test_suites.test_cases]]
flow_node_id = "Template Connector Call"
initial_variables = """
{"username": "nilptr", "token": "very_secret_token", "message": "fail - will this message reach Zeebe?"}
"""
expected_variables = """
{"message": "fail - will this message reach Zeebe?", "token": "very_secret_token", "username": "nilptr"}
"""
`
	var conf Config
	_, err := toml.Decode(tomlData, &conf)
	if err != nil {
		t.Fatalf("Failed to decode TOML config: %v", err)
	}

	if len(conf.TestSuites) != 1 {
		t.Fatalf("Expected 1 test suite, got %d", len(conf.TestSuites))
	}

	ts := conf.TestSuites[0]
	ts.MessageKey = processUniqueMarkers(ts.MessageKey).(string)
	if strings.Contains(ts.MessageKey, "#unique") {
		t.Errorf("Expected message_key substitution, but got: %s", ts.MessageKey)
	}

	if len(ts.TestCases) != 1 {
		t.Fatalf("Expected 1 test case, got %d", len(ts.TestCases))
	}

	tc := ts.TestCases[0]
	// Check that initial_variables were decoded correctly.
	if tc.InitialVariables["username"] != "nilptr" {
		t.Errorf("Expected username 'nilptr', got: %v", tc.InitialVariables["username"])
	}
}
