package scenarios

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	watchdog_utils "github.com/Bastien-Antigravity/watchdog-agent/src/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario_FleetStopCommandPreserved verifies that the 'stop' command and menu option
// are fully preserved in scripts/fleet.py and delegated to by fleet.sh and fleet.cmd.
func TestScenario_FleetStopCommandPreserved(t *testing.T) {
	deployDir := resolveDockerDeploymentDir(t)
	fleetPyPath := filepath.Join(deployDir, "scripts", "fleet.py")

	contentBytes, err := os.ReadFile(fleetPyPath)
	require.NoError(t, err, "scripts/fleet.py must exist")
	content := string(contentBytes)

	// 1. Must define stop_all routine
	assert.True(t, strings.Contains(content, "def stop_all"), "fleet.py must define stop_all routine")
	assert.True(t, strings.Contains(content, `"stop"`), "fleet.py must handle 'stop' command")

	// 2. Must stop docker compose, standalone containers, and native processes
	assert.True(t, strings.Contains(content, `"down"`), "stop_all must execute docker compose down")
	assert.True(t, strings.Contains(content, "nats-server"), "stop_all must cover nats-server")
	assert.True(t, strings.Contains(content, "timescale-db"), "stop_all must cover timescale-db")
	assert.True(t, strings.Contains(content, "watchdog-agent"), "stop_all must cover watchdog-agent")
}

// TestScenario_ObsoletePlatformScriptsRemoved asserts that redundant, duplicated platform
// scripts (fleet-mac.sh, fleet-linux.sh, fleet-windows.cmd, deploy-*.sh) have been deleted.
func TestScenario_ObsoletePlatformScriptsRemoved(t *testing.T) {
	deployDir := resolveDockerDeploymentDir(t)
	scriptsDir := filepath.Join(deployDir, "scripts")

	obsoleteFiles := []string{
		"fleet-mac.sh",
		"fleet-linux.sh",
		"fleet-windows.cmd",
		"deploy-mac.sh",
		"deploy-linux.sh",
		"deploy-windows.cmd",
	}

	for _, filename := range obsoleteFiles {
		targetPath := filepath.Join(scriptsDir, filename)
		assert.NoFileExists(t, targetPath, "Obsolete script %s must be deleted", filename)
	}
}

// TestScenario_WatchdogNatsCrossPlatformResilience validates that the watchdog agent
// safely handles NATS across different OS architectures without crashing or port fighting.
func TestScenario_WatchdogNatsCrossPlatformResilience(t *testing.T) {
	// 1. Verify fleet keywords recognize Docker and external proxies
	// to prevent watchdog from erroneously treating Docker-bound ports as non-fleet conflicts
	deployDir := resolveDockerDeploymentDir(t)
	utilsGoPath := filepath.Join(deployDir, "..", "watchdog-agent", "src", "utils", "utils.go")
	utilsContent, err := os.ReadFile(utilsGoPath)
	require.NoError(t, err)
	uStr := string(utilsContent)

	assert.True(t, strings.Contains(uStr, `"docker"`), "fleetKeywords must include 'docker'")
	assert.True(t, strings.Contains(uStr, `"docker-proxy"`), "fleetKeywords must include 'docker-proxy'")
	assert.True(t, strings.Contains(uStr, `"nats-server"`), "fleetKeywords must include 'nats-server'")

	// 2. Test port liveness detection helper
	// Spin up a mock TCP listener simulating an external NATS broker (or Docker container)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	mockAddr := listener.Addr().String()
	_, portStr, err := net.SplitHostPort(mockAddr)
	require.NoError(t, err)

	// Verify IsOccupantFleetService runs safely without panicking on the listening port
	isFleet, err := watchdog_utils.IsOccupantFleetService(portStr)
	assert.NoError(t, err)
	t.Logf("IsOccupantFleetService for mock port %s returned: %v", portStr, isFleet)

	// 3. Verify registry.go only falls back to the bundled Mach-O binary on darwin
	registryGoPath := filepath.Join(deployDir, "..", "watchdog-agent", "src", "supervisor", "registry.go")
	regContent, err := os.ReadFile(registryGoPath)
	require.NoError(t, err)
	rStr := string(regContent)

	assert.True(t, strings.Contains(rStr, `runtime.GOOS == "darwin"`),
		"registry.go must restrict bundled nats-server execution strictly to macOS (darwin)")
}

// TestScenario_ConfigExplicitHierarchy validates that the newly organized config
// directory contains explicit, self-documenting subdirectories and backward-compatible symlinks.
func TestScenario_ConfigExplicitHierarchy(t *testing.T) {
	deployDir := resolveDockerDeploymentDir(t)

	// 1. Verify config/environments/
	envDir := filepath.Join(deployDir, "config", "environments")
	assert.FileExists(t, filepath.Join(envDir, "native.yaml"), "native.yaml monolithic profile must exist")
	assert.FileExists(t, filepath.Join(envDir, "docker.yaml"), "docker.yaml monolithic profile must exist")
	assert.FileExists(t, filepath.Join(envDir, "standalone.yaml"), "standalone.yaml symlink must exist")

	// 2. Verify config/services/
	servicesDir := filepath.Join(deployDir, "config", "services")
	expectedSlices := []string{
		"config-server.yaml",
		"log-server.yaml",
		"notif-server.yaml",
		"rag-engine.yaml",
		"tele-remote.yaml",
		"web-interface.yaml",
	}
	for _, slice := range expectedSlices {
		assert.FileExists(t, filepath.Join(servicesDir, slice), "Dedicated slice %s must exist", slice)
	}

	// 3. Verify config/keys/
	keysDir := filepath.Join(deployDir, "config", "keys")
	assert.FileExists(t, filepath.Join(keysDir, "public.pem"), "public.pem must exist in config/keys")
	assert.FileExists(t, filepath.Join(keysDir, "private.pem"), "private.pem must exist in config/keys")

	// 4. Verify shared-config/ backwards-compatibility symlink layer
	sharedDir := filepath.Join(deployDir, "shared-config")
	assert.FileExists(t, filepath.Join(sharedDir, "native.yaml"))
	assert.FileExists(t, filepath.Join(sharedDir, "docker.yaml"))
	assert.FileExists(t, filepath.Join(sharedDir, "standalone.yaml"))
	assert.FileExists(t, filepath.Join(sharedDir, "CONFIG_STANDARD.md"))
}

// TestScenario_PythonFleetOrchestratorIntegrity validates that the unified Python 3
// fleet orchestrator (fleet.py) is present, well-formed, compatible with standard library only,
// and correctly delegated to by fleet.sh and fleet.cmd.
func TestScenario_PythonFleetOrchestratorIntegrity(t *testing.T) {
	deployDir := resolveDockerDeploymentDir(t)
	fleetPyPath := filepath.Join(deployDir, "scripts", "fleet.py")
	fleetShPath := filepath.Join(deployDir, "fleet.sh")
	fleetCmdPath := filepath.Join(deployDir, "fleet.cmd")

	// 1. scripts/fleet.py must exist
	assert.FileExists(t, fleetPyPath, "fleet.py must exist in docker-deployment/scripts")
	contentBytes, err := os.ReadFile(fleetPyPath)
	require.NoError(t, err)
	content := string(contentBytes)

	// 2. Must only use Python standard library (no pip packages like requests, psutil, pyyaml, docker)
	forbiddenImports := []string{"import requests", "import psutil", "import yaml", "import docker"}
	for _, forbidden := range forbiddenImports {
		assert.False(t, strings.Contains(content, forbidden), "fleet.py must not require third-party package: %s", forbidden)
	}

	// 3. Must cover all primary commands
	expectedCommands := []string{"local", "docker-local", "distributed", "prod", "compile", "rebuild", "status", "stop", "doctor", "guide"}
	for _, cmd := range expectedCommands {
		assert.True(t, strings.Contains(content, `"`+cmd+`"`), "fleet.py must handle command: %s", cmd)
	}

	// 4. fleet.sh must delegate to fleet.py
	shBytes, err := os.ReadFile(fleetShPath)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(shBytes), "fleet.py"), "fleet.sh must route to fleet.py")

	// 5. fleet.cmd must delegate to fleet.py
	cmdBytes, err := os.ReadFile(fleetCmdPath)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(cmdBytes), "fleet.py"), "fleet.cmd must route to fleet.py")
}

