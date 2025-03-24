package app

type TestCase struct {
	FlowNodeID        string                 `toml:"flow_node_id"`
	ExpectedVariables map[string]interface{} `toml:"expected_variables"`
	InitialVariables  map[string]interface{} `toml:"initial_variables"`
}

type TestSuite struct {
	ProcessID string     `toml:"process_id"`
	TestCases []TestCase `toml:"test_cases"`
}

type Config struct {
	TestSuites []TestSuite `toml:"test_suites"`
}
