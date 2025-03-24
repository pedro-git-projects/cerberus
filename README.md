# Flow Sentry Configuration & CLI Documentation

Flow Sentry uses a configurable setup to connect to Zeebe, Operate, and related services. Configuration options can be set via **command-line flags** or **environment variables**. If no values are provided, the program will fall back to sensible defaults.

---

## ⚙️ Configuration Options

The configuration is defined in a `config` struct with the following fields:

| Field                      | Default Value                                                                                                                  | Description                                                                                         |
|----------------------------|--------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| **ClientID**               | `zeebe`                                                                                                                        | The client identifier used for OAuth.                                                              |
| **ClientSecret**           | `zecret`                                                                                                                       | The client secret used for OAuth.                                                                  |
| **Audience**               | `zeebe-api`                                                                                                                    | The audience for OAuth tokens.                                                                     |
| **GatewayAddress**         | `localhost:26500` (unless overridden via flag or env var)                                                                      | The address of the Zeebe gateway.                                                                  |
| **AuthorizationServerURL** | `http://localhost:18080/auth/realms/camunda-platform/protocol/openid-connect/token`                                            | The URL of the authorization server used to obtain OAuth tokens.                                  |
| **OperateBaseURL**         | `http://localhost:8081`                                                                                                        | The base URL for the Operate service.                                                              |
| **BpmnPath**               | OS-specific default:<br>• Linux/Mac: `$HOME/.config/flow-sentry/workflows/connector_test.bpmn`<br>• Windows: `%AppData%\local\flow-sentry\workflows\connector_test.bpmn` | The default BPMN file to deploy.                                                                  |
| **SuitesPath**             | OS-specific default:<br>• Linux/Mac: `$HOME/.config/flow-sentry/suites`<br>• Windows: `%AppData%\local\flow-sentry\suites`     | The directory where test suite `.toml` files are stored.                                          |
| **TestSuitesFlag**         | `all`                                                                                                                          | Which test suites to run: `all`, a specific ID, or a comma-separated list.                         |
| **DeployWorkflowsFlag**    | `none`                                                                                                                         | Workflow deployment behavior: `none`, `suite`, `all`, or list of BPMN files.                       |

---

## 🚀 CLI Usage

```bash
./flow-sentry [flags]
```

### Default behavior (no flags):
- ✅ Run **all test suites** in the default suites directory  
- 🚫 **Do not deploy** any workflows

---

## 🏁 Command-Line Flags

### `-testsuites`

Run one or more test suites, identified by their `process_id`.

- **Type:** `string`
- **Default:** `"all"`
- **Accepted values:**
  - `all`: Run all test suites found in the suite directory
  - `foo`: Run the suite with `process_id = "foo"`
  - `foo,bar`: Run multiple suites

**Example:**

```bash
./flow-sentry -testsuites=invoice-process
```

---

### `-deployWorkflows`

Deploy BPMN workflows before running test suites.

- **Type:** `string`
- **Default:** `"none"`
- **Accepted values:**
  - `none`: Do not deploy anything
  - `suite`: Deploy workflows listed in each test suite's `workflow_deploy` field
  - `all`: Deploy all `.bpmn` files from the BPMN directory
  - `file1.bpmn,file2.bpmn`: Deploy only the specified workflows (must exist in BPMN folder)

**Examples:**

```bash
./flow-sentry -deployWorkflows=suite
./flow-sentry -deployWorkflows=invoice.bpmn,shipment.bpmn
```

---

## 📁 File Path Defaults

### BPMN File Path

If not configured manually, the default BPMN path is:

- **Linux/macOS:**  
  `~/.config/flow-sentry/workflows/connector_test.bpmn`

- **Windows:**  
  `%AppData%\local\flow-sentry\workflows\connector_test.bpmn`

### Test Suites Path

If not configured manually, the default test suite folder is:

- **Linux/macOS:**  
  `~/.config/flow-sentry/suites`

- **Windows:**  
  `%AppData%\local\flow-sentry\suites`

---

## 🌿 Environment Variables

Instead of passing flags, you can use environment variables to configure Flow Sentry:

| Variable                    | Overrides Field             |
|----------------------------|-----------------------------|
| `OPERATE_BASE_URL`         | `OperateBaseURL`            |
| `AUTHORIZATION_SERVER_URL` | `AuthorizationServerURL`    |
| `GATEWAY_ADDRESS`          | `GatewayAddress`            |

**Examples:**

```bash
export OPERATE_BASE_URL="http://example.com:8081"
export AUTHORIZATION_SERVER_URL="http://example.com/auth/realms/yourrealm/protocol/openid-connect/token"
export GATEWAY_ADDRESS="example.com:26500"
```

**Note:** If both environment variables and flags are provided, **flags take precedence**.

---

## 🎯 Example Workflows

| Command                                                | Behavior                                                 |
|--------------------------------------------------------|----------------------------------------------------------|
| `./flow-sentry`                                        | ✅ Run all test suites<br>🚫 Do not deploy workflows      |
| `./flow-sentry -testsuites=all -deployWorkflows=all`   | ✅ Run all test suites<br>✅ Deploy all BPMNs             |
| `./flow-sentry -testsuites=invoice-process`            | ✅ Run only that suite<br>🚫 Do not deploy workflows      |
| `./flow-sentry -deployWorkflows=suite`                 | ✅ Run all suites<br>✅ Deploy workflows listed in suites |
| `./flow-sentry -testsuites=foo,bar -deployWorkflows=foo.bpmn` | ✅ Run foo + bar suites<br>✅ Deploy foo.bpmn     |

---

![test runner](images/print.jpg)
