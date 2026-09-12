package scenarios

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	distconf_core "github.com/Bastien-Antigravity/distributed-config/src/core"
	distconf_loader "github.com/Bastien-Antigravity/distributed-config/src/loader"
	"github.com/Bastien-Antigravity/distributed-config/src/secret"
	distconf_utils "github.com/Bastien-Antigravity/distributed-config/src/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// resolveDockerDeploymentDir finds the docker-deployment directory relative to this test file.
func resolveDockerDeploymentDir(t *testing.T) string {
	// Starting from current dir, look for docker-deployment in parent dirs
	cwd, err := os.Getwd()
	require.NoError(t, err)

	candidate := filepath.Join(cwd, "..", "..", "..", "docker-deployment")
	absPath, err := filepath.Abs(candidate)
	require.NoError(t, err)

	if _, err := os.Stat(filepath.Join(absPath, "docker-compose.yaml")); err == nil {
		return absPath
	}

	t.Fatalf("Could not locate docker-deployment directory from %s", cwd)
	return ""
}

// TestScenario_DockerServiceSlicesIntegrity validates that all dedicated per-service
// YAML slices follow the ecosystem standard: minimal, isolated, valid YAML, and loadable.
func TestScenario_DockerServiceSlicesIntegrity(t *testing.T) {
	deployDir := resolveDockerDeploymentDir(t)
	servicesDir := filepath.Join(deployDir, "config", "services")

	expectedSlices := map[string]struct {
		serviceKey   string
		shouldOmitLS bool
		hasSecrets   bool
	}{
		"config-server.yaml": {serviceKey: "config_server", shouldOmitLS: false, hasSecrets: false},
		"tele-remote.yaml":   {serviceKey: "tele_remote", shouldOmitLS: true, hasSecrets: true},
		"notif-server.yaml":  {serviceKey: "notif_server", shouldOmitLS: true, hasSecrets: true},
		"web-interface.yaml": {serviceKey: "web_interface", shouldOmitLS: true, hasSecrets: false},
		"rag-engine.yaml":    {serviceKey: "rag_engine", shouldOmitLS: true, hasSecrets: true},
		"log-server.yaml":    {serviceKey: "log_server", shouldOmitLS: false, hasSecrets: false},
	}

	for filename, meta := range expectedSlices {
		t.Run("Slice_"+filename, func(t *testing.T) {
			filePath := filepath.Join(servicesDir, filename)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Config slice file %s must exist", filename)

			// 1. Must parse as valid YAML
			var node yaml.Node
			err = yaml.Unmarshal(content, &node)
			require.NoError(t, err, "File %s must be valid YAML", filename)

			var rawMap map[string]any
			err = yaml.Unmarshal(content, &rawMap)
			require.NoError(t, err)

			// 2. Must contain common section
			commonVal, hasCommon := rawMap["common"]
			require.True(t, hasCommon, "File %s must contain 'common' section", filename)
			commonMap, ok := commonVal.(map[string]any)
			require.True(t, ok)
			assert.NotEmpty(t, commonMap["name"], "common.name must be set in %s", filename)

			// 3. Must contain config_server section
			csVal, hasCS := rawMap["config_server"]
			require.True(t, hasCS, "File %s must contain 'config_server' section", filename)
			csMap, ok := csVal.(map[string]any)
			require.True(t, ok)
			assert.NotEmpty(t, csMap["ip"], "config_server.ip must be defined in %s", filename)

			// 4. Dedicated service section check
			_, hasServiceKey := rawMap[meta.serviceKey]
			assert.True(t, hasServiceKey, "File %s must define its service key '%s'", filename, meta.serviceKey)

			// 5. Omission of unnecessary log_server section (handled dynamically via sync/defaults)
			if meta.shouldOmitLS {
				_, hasLS := rawMap["log_server"]
				assert.False(t, hasLS, "Consumer service %s must NOT include static 'log_server' block (resolved dynamically)", filename)
			}

			// 6. Zero-Knowledge validation: web-interface and config-server must not hold decrypted secrets
			if filename == "web-interface.yaml" {
				for k := range rawMap {
					assert.NotContains(t, []string{"tele_remote", "notif_server", "timescale_db"}, k,
						"web-interface.yaml must not contain other services' config sections")
				}
			}

			// 7. Test loading via distributed-config loader
			cfg := &distconf_core.Config{Logger: distconf_utils.EnsureSafeLogger(nil)}
			err = distconf_loader.LoadConfigFromFile(cfg, filePath)
			assert.NoError(t, err, "distributed-config loader must successfully load %s", filename)
			assert.NotEmpty(t, cfg.Common.Name)
		})
	}
}

// TestScenario_DockerComposeStandardCompliance validates that docker-compose.yaml
// conforms to standard cross-platform conventions (127.0.0.1 default loopback,
// read-only key mounts for consumers, zero key mounts for web-interface/config-server).
func TestScenario_DockerComposeStandardCompliance(t *testing.T) {
	deployDir := resolveDockerDeploymentDir(t)
	composePath := filepath.Join(deployDir, "docker-compose.yaml")

	content, err := os.ReadFile(composePath)
	require.NoError(t, err)

	var composeMap struct {
		Services map[string]struct {
			Image       string   `yaml:"image"`
			Build       any      `yaml:"build"`
			Ports       []string `yaml:"ports"`
			Volumes     []string `yaml:"volumes"`
			Environment []string `yaml:"environment"`
		} `yaml:"services"`
		Networks map[string]any `yaml:"networks"`
		Volumes  map[string]any `yaml:"volumes"`
	}

	err = yaml.Unmarshal(content, &composeMap)
	require.NoError(t, err, "docker-compose.yaml must be valid YAML")

	// 1. Required core services
	requiredServices := []string{
		"tele-remote",
		"notif-server",
		"web-interface",
		"config-server",
		"log-server",
		"rag-engine",
		"nats-server",
		"timescale-db",
	}

	for _, svc := range requiredServices {
		_, exists := composeMap.Services[svc]
		assert.True(t, exists, "Service %s must be declared in docker-compose.yaml", svc)
	}

	// 2. Secret isolation boundary in volumes
	secretConsumers := map[string]bool{
		"tele-remote":  true,
		"notif-server": true,
		"rag-engine":   true,
	}

	for svcName, svc := range composeMap.Services {
		hasKeyMount := false
		isReadOnly := false
		for _, v := range svc.Volumes {
			if strings.Contains(v, "private.pem") {
				hasKeyMount = true
				if strings.HasSuffix(v, ":ro") {
					isReadOnly = true
				}
			}
		}

		if secretConsumers[svcName] {
			assert.True(t, hasKeyMount, "Service %s must mount private.pem for on-demand decryption", svcName)
			assert.True(t, isReadOnly, "Service %s private.pem mount must be read-only (:ro)", svcName)
		} else {
			assert.False(t, hasKeyMount, "Zero-Knowledge Violation: Service %s must NOT mount private.pem", svcName)
		}

		// 3. Port bindings must use standard portable HOST_IP variable defaulting to 127.0.0.1
		for _, portMapping := range svc.Ports {
			if strings.Contains(portMapping, "HOST_IP") {
				assert.Contains(t, portMapping, "${HOST_IP:-127.0.0.1}",
					"Port mapping %s in %s should default HOST_IP to 127.0.0.1 for standard cross-platform compatibility",
					portMapping, svcName)
			}
		}
	}
}

// TestScenario_DockerCrossPlatformKeyFallback validates that the repository
// provides a self-contained fallback key pair in config/keys/ allowing any machine
// to run the stack immediately without missing-file errors.
func TestScenario_DockerCrossPlatformKeyFallback(t *testing.T) {
	deployDir := resolveDockerDeploymentDir(t)
	privKeyPath := filepath.Join(deployDir, "config", "keys", "private.pem")
	pubKeyPath := filepath.Join(deployDir, "config", "keys", "public.pem")

	// 1. Files must exist
	privBytes, err := os.ReadFile(privKeyPath)
	require.NoError(t, err, "Fallback private key config/keys/private.pem must exist")
	assert.NotEmpty(t, privBytes)
	pubBytes, err := os.ReadFile(pubKeyPath)
	require.NoError(t, err, "Fallback public key config/keys/public.pem must exist")

	// 2. Cryptographic roundtrip test with fallback keys
	testSecret := "docker-portable-secret-test-string"
	encVal, err := secret.Encrypt(testSecret, string(pubBytes))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encVal, "ENC("))

	t.Setenv("BASTIEN_PRIVATE_KEY_PATH", privKeyPath)
	decVal, err := secret.Decrypt(encVal)
	require.NoError(t, err)
	assert.Equal(t, testSecret, decVal, "Decrypted secret must match original using repository fallback key")
}
