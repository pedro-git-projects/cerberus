package app

type APICall struct {
	// Endpoint to call (absolute or relative).
	Endpoint string `toml:"endpoint" json:"endpoint"`
	// HTTP method to use (e.g., GET, POST).
	Method string `toml:"method" json:"method"`
	// Payload for the API call.
	Payload JSONMap `toml:"payload" json:"payload"`
	// Optional headers.
	Headers JSONMap `toml:"headers,omitempty" json:"headers,omitempty"`
}
