# Flow Sentry Configuration Documentation

Flow Sentry uses a configurable setup to connect to Zeebe, Operate, and related services. Configuration options can be set via command-line flags or environment variables. If no values are provided, the program will fall back to sensible defaults.

## Configuration Options

The configuration is defined in a `config` struct with the following fields:

| Field                      | Default Value                                                                                                                  | Description                                                                                         |
|----------------------------|--------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| **ClientID**               | `zeebe`                                                                                                                        | The client identifier used for OAuth.                                                             |
| **ClientSecret**           | `zecret`                                                                                                                       | The client secret used for OAuth.                                                                 |
| **Audience**               | `zeebe-api`                                                                                                                    | The audience for OAuth tokens.                                                                    |
| **GatewayAddress**         | `localhost:26500` (unless overridden via flag or env var)                                                                      | The address of the Zeebe gateway.                                                                 |
| **AuthorizationServerURL** | `http://localhost:18080/auth/realms/camunda-platform/protocol/openid-connect/token` (unless overridden)                           | The URL of the authorization server used to obtain OAuth tokens.                                  |
| **OperateBaseURL**         | `http://localhost:8081` (unless overridden)                                                                                      | The base URL for the Operate service.                                                             |
| **BpmnPath**               | OS-specific default:<br>• Linux/Mac: `$HOME/.config/flows-sentry/workflows/connector_test.bpmn`<br>• Windows: `%AppData%\local\flow-sentry\workflows\connector_test.bpmn` | The file path to the BPMN file to deploy.                                                         |
| **SuitesPath**             | OS-specific default:<br>• Linux/Mac: `$HOME/.config/flows-sentry/suites`<br>• Windows: `%AppData%\local\flow-sentry\suites`     | The directory path for test suites.                                                               |

## Command-Line Flags

The program supports the following flags to override default values:

- `-operate`: Specifies the Operate base URL.  
  _Example:_ `-operate=http://example.com:8081`
  
- `-auth`: Specifies the authorization server URL.  
  _Example:_ `-auth=http://example.com/auth/realms/yourrealm/protocol/openid-connect/token`
  
- `-gateway`: Specifies the Zeebe gateway address.  
  _Example:_ `-gateway=example.com:26500`

Usage (when running the program):

```bash
./flow-sentry -operate=http://example.com:8081 -auth=http://example.com/auth -gateway=example.com:26500
```

## Environment Variables

Alternatively, you can set environment variables to override the defaults. The program checks for these variables before falling back to the hard-coded defaults.

- **OPERATE_BASE_URL**  
  _Example:_
  ```bash
  export OPERATE_BASE_URL="http://example.com:8081"
  ```
  
- **AUTHORIZATION_SERVER_URL**  
  _Example:_
  ```bash
  export AUTHORIZATION_SERVER_URL="http://example.com/auth/realms/yourrealm/protocol/openid-connect/token"
  ```
  
- **GATEWAY_ADDRESS**  
  _Example:_
  ```bash
  export GATEWAY_ADDRESS="example.com:26500"
  ```

When both a flag and an environment variable are provided, the command-line flag will take precedence.

## File Path Configuration

### BPMN File Path

- **Linux/Mac:**  
  If no BPMN path is provided, it defaults to:  
  `~/.config/flows-sentry/workflows/connector_test.bpmn`
  
- **Windows:**  
  If no BPMN path is provided, it defaults to:  
  `%AppData%\local\flow-sentry\workflows\connector_test.bpmn`

### Test Suites Path

- **Linux/Mac:**  
  Defaults to:  
  `~/.config/flows-sentry/suites`
  
- **Windows:**  
  Defaults to:  
  `%AppData%\local\flow-sentry\suites`

These paths are automatically determined based on the OS. If you wish to deploy BPMNs from a different location or manage test suites in another directory, modify the respective functions in the code or set the values manually if future options are added.

## Summary

1. **Defaults:**  
   The program has sensible defaults for most settings. For example, it uses `localhost:26500` for the gateway, and standard paths under the user's home directory for BPMN and test suite files.
   
2. **Overrides:**  
   You can override these defaults using:
   - Command-line flags (`-operate`, `-auth`, and `-gateway`).
   - Environment variables (`OPERATE_BASE_URL`, `AUTHORIZATION_SERVER_URL`, `GATEWAY_ADDRESS`).

3. **Execution:**  
   The configuration is parsed at startup. Make sure you set the appropriate values before running the program.

By following these guidelines, you can flexibly configure the Flow Sentry program to suit your environment and deployment scenario.


## Testing Ports:

### Zeebe:

- gRPC and related API on port 26500
- Health check on port 9600
- (Additional HTTP interface on 8088)

### Operate:

- Main interface on port 8081

### Tasklist:

- Accessible on port 8082

### Connectors:

- Accessible on port 8085

### Optimize:

- Accessible on port 8083 (internally runs on 8090)

### Identity:

- Accessible on port 8084

### Keycloak:

- Accessible on port 18080

## Credentials

### Operate: 

- username: demo

- password: demo

### Keycloak

- username: admin

- password: admin

## Audiences

### Zeebe:

- zeebe-api

### Operate:

- operate-api

### Optimize:

- optimize-api

### Tasklist:

- tasklist-api

These values are then used by Identity to validate tokens and are embedded in the `JWT’s` aud claim.

## Template Directory on Linux

`/opt/camunda-modeler/resources/element-templates`


![test runner](images/print.jpg)
