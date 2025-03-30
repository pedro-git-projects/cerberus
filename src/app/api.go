package app

type APICall struct {
	// Endpoint to call (absolute or relative).
	Endpoint string  `toml:"endpoint" json:"endpoint"`
	Method   string  `toml:"method" json:"method"`
	Payload  JSONMap `toml:"payload" json:"payload"`
	Headers  JSONMap `toml:"headers,omitempty" json:"headers,omitempty"`
}

// TOML Example
// [test_suites.api_call]
// endpoint = "http://localhost:6969/echo"
// method = "POST"
// payload = '{"hello": "world", "from": "flow-sentry"}'
// headers = '{"Content-Type": "application/json"}'
