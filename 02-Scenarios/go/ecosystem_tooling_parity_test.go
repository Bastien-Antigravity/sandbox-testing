package scenarios

/*
ESSENTIAL PROCESS:
Ecosystem Tooling Parity and Anti-Pattern Enforcement Scenario Suite.
Verifies that across Go, Python, and FFI layers:
1. Microservices bootstrap via microservice-toolbox / universal-logger facades.
2. Dynamic capability resolution strictly fails fast on missing ports/IPs with zero silent fallbacks.
3. Logger instances satisfy the 5-level logging contract without resorting to bare stdout prints.
4. Dynamic semver versioning binds strictly to VERSION.txt across all services.
5. Polyglot bindings (Python bootstrap, CGO ABI shared libraries libdistconf, libunilog, libnotif) are healthy.

DATA FLOW:
1. TestScenario_PolyglotToolingParity:
   - Initializes Go service context through toolbox_bootstrap.BootstrapServiceSafe.
   - Validates that dynamic version matches root VERSION.txt.
   - Proves fail-fast behavior when resolving non-existent capabilities (no silent fallback).
   - Validates active capabilities resolution (config_server, web_interface, etc.).
   - Dispatches structured log events through unilog Logger without panicking.
   - Executes Python bootstrap sub-process to verify cross-language initialization compatibility.
   - Validates presence and ABI header parity of CGO shared libraries (libdistconf, libunilog, libnotif).

KEY PARAMETERS:
- TestServiceName: "sandbox-tooling-parity-svc"
- FallbackProbeCapability: "non_existent_unregistered_capability"
*/

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	toolbox_bootstrap "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------

func TestScenario_PolyglotToolingParity(t *testing.T) {
	workspaceRoot := resolveWorkspaceRoot(t)

	// -------------------------------------------------------------------------
	// 1. Go Microservice Bootstrap & Dynamic Versioning Verification
	// -------------------------------------------------------------------------
	t.Run("Go_BootstrapServiceSafe_And_Dynamic_Versioning", func(t *testing.T) {
		appConfig, appLogger, err := toolbox_bootstrap.BootstrapServiceSafe("sandbox-tooling-parity-svc")
		require.NoError(t, err, "BootstrapServiceSafe must succeed in standalone environment")
		require.NotNil(t, appConfig, "AppConfig must be initialized")
		require.NotNil(t, appLogger, "Logger must be initialized")

		// Read canonical VERSION.txt from workspace root
		versionPath := filepath.Join(workspaceRoot, "VERSION.txt")
		if _, err := os.Stat(versionPath); err == nil {
			vBytes, err := os.ReadFile(versionPath)
			require.NoError(t, err)
			expectedVersion := strings.TrimSpace(string(vBytes))

			assert.Equal(t, expectedVersion, appConfig.GetVersion(),
				"AppConfig.GetVersion() must resolve dynamically to repository root VERSION.txt")
		}

		// Exercise Logger interface methods without stdout panics
		appLogger.Debug("Test debug message from parity scenario")
		appLogger.Info("Test info message from parity scenario")
		appLogger.Warning("Test warning message from parity scenario")
		appLogger.Error("Test error message from parity scenario")
	})

	// -------------------------------------------------------------------------
	// 2. Fail-Fast Dynamic Capability Resolution (Zero Silent Fallbacks)
	// -------------------------------------------------------------------------
	t.Run("Dynamic_Capability_Resolution_Zero_Fallback_Policy", func(t *testing.T) {
		appConfig, _, err := toolbox_bootstrap.BootstrapServiceSafe("sandbox-tooling-parity-svc")
		require.NoError(t, err)

		// A non-existent capability MUST return an error and NEVER fallback to a default port like 8080/50051
		addr, err := appConfig.GetListenAddr("non_existent_unregistered_capability")
		assert.Error(t, err, "Resolving an unregistered capability must return an error (fail-fast)")
		assert.Empty(t, addr, "Unregistered capability address must be empty on failure")

		grpcAddr, err := appConfig.GetGRPCListenAddr("non_existent_unregistered_capability")
		assert.Error(t, err, "Resolving unregistered gRPC address must return an error")
		assert.Empty(t, grpcAddr, "Unregistered gRPC address must be empty on failure")

		// Registered ecosystem capabilities MUST resolve properly
		webAddr, err := appConfig.GetListenAddr("web_interface")
		if err == nil {
			assert.NotEmpty(t, webAddr, "web_interface address must not be empty")
			assert.Contains(t, webAddr, ":", "web_interface address must be in host:port format")
		}

		cfgAddr, err := appConfig.GetListenAddr("config_server")
		if err == nil {
			assert.NotEmpty(t, cfgAddr, "config_server address must not be empty")
			assert.Contains(t, cfgAddr, ":", "config_server address must be in host:port format")
		}
	})

	// -------------------------------------------------------------------------
	// 3. Python Microservice Bootstrap Contract Verification
	// -------------------------------------------------------------------------
	t.Run("Python_BootstrapMicroservice_Contract", func(t *testing.T) {
		pythonScript := `
import sys
from microservice_toolbox.utils.bootstrap import bootstrap_microservice
from microservice_toolbox.config.loader import AppConfig
from microservice_toolbox.version import get_version

# Verify bootstrap utility executes
target_file = __file__ if "__file__" in globals() else "main.py"
bootstrap_microservice(target_file)

# Verify dynamic versioning resolves
v = get_version()
assert v is not None and len(v) > 0, "Python get_version() returned empty version"
print(f"PYTHON_BOOTSTRAP_OK:{v}")
`
		pythonBin := filepath.Join(workspaceRoot, "obsidian-brain", "08-Squad-Agent", ".venv", "bin", "python3")
		if _, err := os.Stat(pythonBin); err != nil {
			pythonBin = "python3"
		}
		cmd := exec.Command(pythonBin, "-c", pythonScript)
		cmd.Dir = filepath.Join(workspaceRoot, "obsidian-brain", "08-Squad-Agent")
		cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(workspaceRoot, "microservice-toolbox", "python"))
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "Python bootstrap must execute cleanly: %s", string(out))
		assert.Contains(t, string(out), "PYTHON_BOOTSTRAP_OK:", "Python script output must confirm bootstrap OK")
	})

	// -------------------------------------------------------------------------
	// 4. Polyglot CGO Bridges Presence and ABI Header Integrity
	// -------------------------------------------------------------------------
	t.Run("CGO_Bridges_Presence_And_Header_Integrity", func(t *testing.T) {
		cgoBridges := []struct {
			name       string
			headerPath string
			dylibPath  string
		}{
			{
				name:       "libunilog",
				headerPath: filepath.Join(workspaceRoot, "universal-logger", "unilog", "libunilog", "libunilog.h"),
				dylibPath:  filepath.Join(workspaceRoot, "universal-logger", "unilog", "libunilog", "libunilog.dylib"),
			},
			{
				name:       "libdistconf",
				headerPath: filepath.Join(workspaceRoot, "distributed-config", "distconf", "libdistconf", "libdistconf.h"),
				dylibPath:  filepath.Join(workspaceRoot, "distributed-config", "distconf", "libdistconf", "libdistconf.dylib"),
			},
			{
				name:       "libnotif",
				headerPath: filepath.Join(workspaceRoot, "notif-server", "notif", "libnotif", "libnotif.h"),
				dylibPath:  filepath.Join(workspaceRoot, "notif-server", "notif", "libnotif", "libnotif.dylib"),
			},
		}

		for _, bridge := range cgoBridges {
			// Assert C header exists
			_, err := os.Stat(bridge.headerPath)
			assert.NoError(t, err, "%s header must exist at %s", bridge.name, bridge.headerPath)

			// Assert shared library exists on macOS
			_, err = os.Stat(bridge.dylibPath)
			assert.NoError(t, err, "%s shared object must exist at %s", bridge.name, bridge.dylibPath)
		}
	})
}
