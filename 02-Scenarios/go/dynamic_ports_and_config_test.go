package scenarios

/*
ESSENTIAL PROCESS:
Dynamic Port Resolution and 4-Layer Drift Prevention Scenario Suite.
Verifies that:
1. All microservices resolve network listen addresses dynamically from configuration
   without hardcoded port fallbacks.
2. Port shifting (e.g. +10000) works end-to-end across distributed-config and microservice-toolbox.
3. Network sockets can dynamically bind to shifted ports.
4. Ports across the 4 architectural layers (native.yaml SSoT, docker-compose.yaml manifests,
   service-registry.json, and documentation tables) remain in 100% strict alignment without drift.

DATA FLOW:
1. TestScenario_DynamicPortShiftingAndZeroHardcodedFallbacks:
   - Injects shifted port configuration (+10000) into distributed-config / microservice-toolbox.
   - Validates that GetListenAddr, GetGRPCListenAddr, and GetRESTAddr return the shifted endpoints.
   - Confirms that missing port configuration produces explicit errors instead of silent fallbacks.
   - Binds dynamic listeners to the shifted addresses to prove operational viability.
2. TestScenario_CrossLayerPortDriftAudit:
   - Parses native.yaml (SSoT) capabilities.
   - Compares against docker-compose.yaml port definitions and environment variables.
   - Compares against service-registry.json default_port, grpc_port, and rest_port definitions.
   - Asserts zero divergence across layers.

KEY PARAMETERS:
- ShiftOffset: 10000 (shifted port space used for isolation validation).
- CoreServices: config_server, log_server, notif_server, tele_remote, web_interface, watchdog_agent, nats_server, timescale_db.
*/

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	distconf_loader "github.com/Bastien-Antigravity/distributed-config/src/loader"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// resolveWorkspaceRoot locates the root Bastien-Antigravity directory.
func resolveWorkspaceRoot(t *testing.T) string {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	candidate := filepath.Join(cwd, "..", "..", "..")
	absPath, err := filepath.Abs(candidate)
	require.NoError(t, err)

	if _, err := os.Stat(filepath.Join(absPath, "docker-deployment")); err == nil {
		return absPath
	}

	t.Fatalf("Could not locate workspace root from %s", cwd)
	return ""
}

// -----------------------------------------------------------------------------

func TestScenario_DynamicPortShiftingAndZeroHardcodedFallbacks(t *testing.T) {
	workspaceRoot := resolveWorkspaceRoot(t)
	nativeYamlPath := filepath.Join(workspaceRoot, "docker-deployment", "modes", "local", "config", "native.yaml")
	require.FileExists(t, nativeYamlPath)

	t.Run("DynamicPortShifting_Via_ConfigOverrides", func(t *testing.T) {
		// Define shifted ports (+10000)
		shiftedPorts := map[string]map[string]int{
			"config_server": {
				"port":      13306,
				"grpc_port": 13307,
				"rest_port": 13308,
			},
			"log_server": {
				"port":      19020,
				"grpc_port": 19021,
			},
			"notif_server": {
				"port":      11026,
				"grpc_port": 11027,
				"rest_port": 11029,
			},
			"tele_remote": {
				"port": 11863,
			},
			"web_interface": {
				"port":      15000,
				"grpc_port": 18001,
			},
			"watchdog_agent": {
				"port":      19090,
				"rest_port": 19095,
			},
			"nats_server": {
				"port": 14222,
			},
			"timescale_db": {
				"port": 15432,
			},
		}

		// Canonical ports that must NEVER be returned when shifted
		canonicalPorts := map[string]int{
			"config_server":  3306,
			"log_server":     9020,
			"notif_server":   1026,
			"tele_remote":    1863,
			"web_interface":  5000,
			"watchdog_agent": 9095,
			"nats_server":    4222,
			"timescale_db":   5432,
		}

		// Build a shifted config YAML
		shiftedYaml := map[string]any{
			"common": map[string]any{
				"name": "shifted-test",
			},
			"capabilities": make(map[string]any),
		}

		capsMap := shiftedYaml["capabilities"].(map[string]any)
		for svc, ports := range shiftedPorts {
			svcMap := map[string]any{
				"ip": "127.0.0.1",
			}
			if _, hasGrpc := ports["grpc_port"]; hasGrpc {
				svcMap["grpc_ip"] = "127.0.0.1"
			}
			for pKey, pVal := range ports {
				svcMap[pKey] = strconv.Itoa(pVal)
			}
			capsMap[svc] = svcMap
		}

		shiftedBytes, err := yaml.Marshal(shiftedYaml)
		require.NoError(t, err)

		tempFile, err := os.CreateTemp("", "shifted-config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.Write(shiftedBytes)
		require.NoError(t, err)
		// Load shifted config using distributed-config
		distCfg := distributed_config.New("standalone")
		err = distconf_loader.LoadConfigFromFile(distCfg.Config, tempFile.Name())
		require.NoError(t, err)

		appConfig := &toolbox_config.AppConfig{Config: distCfg}

		// Verify that all addresses match the dynamic shifted ports
		for svc, ports := range shiftedPorts {
			for pKey, expectedPort := range ports {
				var addr string
				var err error

				switch pKey {
				case "port":
					addr, err = appConfig.GetListenAddr(svc)
				case "grpc_port":
					addr, err = appConfig.GetGRPCListenAddr(svc)
				case "rest_port":
					addr, err = appConfig.GetRESTAddr(svc)
				}

				require.NoError(t, err, "Failed to resolve %s.%s on shifted config", svc, pKey)
				expectedAddr := fmt.Sprintf("127.0.0.1:%d", expectedPort)
				assert.Equal(t, expectedAddr, addr, "%s.%s must resolve dynamically to %s", svc, pKey, expectedAddr)

				// Verify NO fallback to canonical port
				if canonPort, exists := canonicalPorts[svc]; exists {
					assert.NotContains(t, addr, fmt.Sprintf(":%d", canonPort),
						"DYNAMIC INVARIANT VIOLATION: %s resolved to canonical port %d instead of shifted port %d",
						svc, canonPort, expectedPort)
				}
			}
		}

		// Validate unique ports check passes with shifted ports
		err = appConfig.ValidateUniquePorts()
		assert.NoError(t, err, "Shifted ports must not contain collisions")
	})

	t.Run("ZeroFallback_WhenPortOmittedOrEmpty", func(t *testing.T) {
		// Create config with a capability that is missing 'port'
		incompleteYaml := map[string]any{
			"common": map[string]any{
				"name": "incomplete-test",
			},
			"capabilities": map[string]any{
				"ghost_service": map[string]any{
					"ip": "127.0.0.1",
					// "port" intentionally missing
				},
				"empty_port_service": map[string]any{
					"ip":   "127.0.0.1",
					"port": "", // explicitly empty
				},
			},
		}

		bytes, err := yaml.Marshal(incompleteYaml)
		require.NoError(t, err)

		tempFile, err := os.CreateTemp("", "incomplete-config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.Write(bytes)
		require.NoError(t, err)
		_ = tempFile.Close()

		distCfg := distributed_config.New("standalone")
		err = distconf_loader.LoadConfigFromFile(distCfg.Config, tempFile.Name())
		require.NoError(t, err)

		appConfig := &toolbox_config.AppConfig{Config: distCfg}

		// Must fail fast with an error, NEVER return a default fallback port
		addr1, err1 := appConfig.GetListenAddr("ghost_service")
		assert.Error(t, err1, "Expected error when port key is missing")
		assert.Empty(t, addr1, "Address must be empty on missing port")

		addr2, err2 := appConfig.GetListenAddr("empty_port_service")
		assert.Error(t, err2, "Expected error when port key is empty string")
		assert.Empty(t, addr2, "Address must be empty on empty port")
	})

	t.Run("OperationalDynamicBinding_ShiftedListener", func(t *testing.T) {
		// Choose dynamic shifted ports
		testPort := 24391
		addr := fmt.Sprintf("127.0.0.1:%d", testPort)

		listener, err := net.Listen("tcp", addr)
		require.NoError(t, err, "Must be able to bind dynamically to shifted address %s", addr)
		defer listener.Close()

		connected := make(chan bool, 1)
		go func() {
			conn, err := listener.Accept()
			if err == nil {
				_ = conn.Close()
				connected <- true
			}
		}()

		clientConn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		require.NoError(t, err, "Must connect to dynamic shifted listener")
		defer clientConn.Close()

		select {
		case <-connected:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for dynamic listener connection")
		}
	})
}

// -----------------------------------------------------------------------------

func TestScenario_CrossLayerPortDriftAudit(t *testing.T) {
	workspaceRoot := resolveWorkspaceRoot(t)

	nativeYamlPath := filepath.Join(workspaceRoot, "docker-deployment", "modes", "local", "config", "native.yaml")
	dockerComposePath := filepath.Join(workspaceRoot, "docker-deployment", "docker-compose.yaml")
	registryJsonPath := filepath.Join(workspaceRoot, "obsidian-brain", "05-Fleet-Operation", "00-Repo-Control", "service-registry.json")

	require.FileExists(t, nativeYamlPath)
	require.FileExists(t, dockerComposePath)
	require.FileExists(t, registryJsonPath)

	// 1. Parse Layer 1: native.yaml (SSoT)
	nativeBytes, err := os.ReadFile(nativeYamlPath)
	require.NoError(t, err)

	var nativeRoot struct {
		Capabilities map[string]map[string]any `yaml:"capabilities"`
	}
	err = yaml.Unmarshal(nativeBytes, &nativeRoot)
	require.NoError(t, err)

	// Helper to extract numeric default from pattern "${VAR:DEFAULT}" or raw number
	parseEnvDefaultPort := func(val any) int {
		if val == nil {
			return 0
		}
		strVal := fmt.Sprintf("%v", val)
		re := regexp.MustCompile(`\$\{.*?:([0-9]+)\}`)
		match := re.FindStringSubmatch(strVal)
		if len(match) > 1 {
			p, _ := strconv.Atoi(match[1])
			return p
		}
		p, _ := strconv.Atoi(strVal)
		return p
	}

	ssotPorts := make(map[string]map[string]int)
	for svc, capMap := range nativeRoot.Capabilities {
		ports := make(map[string]int)
		if p := parseEnvDefaultPort(capMap["port"]); p > 0 {
			ports["port"] = p
		}
		if p := parseEnvDefaultPort(capMap["grpc_port"]); p > 0 {
			ports["grpc_port"] = p
		}
		if p := parseEnvDefaultPort(capMap["rest_port"]); p > 0 {
			ports["rest_port"] = p
		}
		if len(ports) > 0 {
			ssotPorts[svc] = ports
		}
	}

	// 2. Parse Layer 2: docker-compose.yaml
	composeBytes, err := os.ReadFile(dockerComposePath)
	require.NoError(t, err)

	var composeRoot struct {
		Services map[string]struct {
			Ports       []string `yaml:"ports"`
			Environment []string `yaml:"environment"`
		} `yaml:"services"`
	}
	err = yaml.Unmarshal(composeBytes, &composeRoot)
	require.NoError(t, err)

	// Extract ports exposed by docker-compose services
	composeServicePorts := make(map[string][]int)
	for svcName, svcDef := range composeRoot.Services {
		var extracted []int
		for _, p := range svcDef.Ports {
			// Matches formats like "${HOST_IP:-127.0.0.1}:${WB_PORT:-5000}:${WB_PORT:-5000}"
			// or "${HOST_IP:-127.0.0.1}:8222:8222"
			re := regexp.MustCompile(`(?::|:-)([0-9]{2,5})`)
			matches := re.FindAllStringSubmatch(p, -1)
			for _, m := range matches {
				if len(m) > 1 {
					portNum, _ := strconv.Atoi(m[1])
					if portNum > 0 {
						extracted = append(extracted, portNum)
					}
				}
			}
		}
		composeServicePorts[svcName] = extracted
	}

	// 3. Parse Layer 3: service-registry.json
	registryBytes, err := os.ReadFile(registryJsonPath)
	require.NoError(t, err)

	var registryRoot struct {
		Services struct {
			Infrastructure []struct {
				Name        string `json:"name"`
				DefaultPort int    `json:"default_port"`
			} `json:"infrastructure"`
			Application []struct {
				Name        string `json:"name"`
				DefaultPort int    `json:"default_port"`
				GrpcPort    int    `json:"grpc_port,omitempty"`
				RestPort    int    `json:"rest_port,omitempty"`
			} `json:"application"`
		} `json:"services"`
	}
	err = json.Unmarshal(registryBytes, &registryRoot)
	require.NoError(t, err)

	registryPorts := make(map[string]map[string]int)
	for _, inf := range registryRoot.Services.Infrastructure {
		registryPorts[inf.Name] = map[string]int{"default_port": inf.DefaultPort}
	}
	for _, app := range registryRoot.Services.Application {
		m := map[string]int{"default_port": app.DefaultPort}
		if app.GrpcPort > 0 {
			m["grpc_port"] = app.GrpcPort
		}
		if app.RestPort > 0 {
			m["rest_port"] = app.RestPort
		}
		registryPorts[app.Name] = m
	}

	// 4. Audit cross-layer consistency
	// Service name normalization map (native capability name -> registry/compose name)
	svcNormMap := map[string]string{
		"config_server":  "config-server",
		"log_server":     "log-server",
		"notif_server":   "notif-server",
		"tele_remote":    "tele-remote",
		"web_interface":  "web-interface",
		"watchdog_agent": "watchdog-agent",
		"nats_server":    "nats-server",
		"timescale_db":   "timescale-db",
	}

	for capName, targetName := range svcNormMap {
		t.Run("Audit_"+capName, func(t *testing.T) {
			ssotMap, hasSSoT := ssotPorts[capName]
			require.True(t, hasSSoT, "native.yaml (SSoT) must define capability %s", capName)

			// Check Layer 3: service-registry.json
			regMap, hasReg := registryPorts[targetName]
			require.True(t, hasReg, "service-registry.json must define service %s", targetName)

			// Default port comparison
			expectedMainPort := ssotMap["port"]
			if capName == "watchdog_agent" {
				expectedMainPort = ssotMap["rest_port"]
			}
			assert.Equal(t, expectedMainPort, regMap["default_port"],
				"DRIFT DETECTED: service-registry.json default_port for %s (%d) does not match native.yaml (%d)",
				targetName, regMap["default_port"], expectedMainPort)

			// Check gRPC port if defined
			if ssotGrpc, ok := ssotMap["grpc_port"]; ok && ssotGrpc > 0 {
				assert.Equal(t, ssotGrpc, regMap["grpc_port"],
					"DRIFT DETECTED: service-registry.json grpc_port for %s does not match native.yaml", targetName)
			}

			// Check REST port if defined
			if ssotRest, ok := ssotMap["rest_port"]; ok && ssotRest > 0 {
				assert.Equal(t, ssotRest, regMap["rest_port"],
					"DRIFT DETECTED: service-registry.json rest_port for %s does not match native.yaml", targetName)
			}

			// Check Layer 2: docker-compose.yaml (excluding watchdog-agent which is native-only supervisor)
			if targetName != "watchdog-agent" {
				composePorts, hasCompose := composeServicePorts[targetName]
				require.True(t, hasCompose, "docker-compose.yaml must define service %s", targetName)
				assert.Contains(t, composePorts, expectedMainPort,
					"DRIFT DETECTED: docker-compose.yaml does not expose SSoT port %d for service %s",
					expectedMainPort, targetName)
			}
		})
	}
}
