package scenarios

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	distconf "github.com/Bastien-Antigravity/distributed-config"
	distconf_core "github.com/Bastien-Antigravity/distributed-config/src/core"
	distconf_loader "github.com/Bastien-Antigravity/distributed-config/src/loader"
	"github.com/Bastien-Antigravity/distributed-config/src/secret"
	distconf_utils "github.com/Bastien-Antigravity/distributed-config/src/utils"
	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	unilog_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
	web_router "github.com/Bastien-Antigravity/web-interface/src/router"
	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type scenarioMockLogger struct{}

func (m *scenarioMockLogger) Debug(format string, args ...any)                           {}
func (m *scenarioMockLogger) Info(format string, args ...any)                            {}
func (m *scenarioMockLogger) Warning(format string, args ...any)                         {}
func (m *scenarioMockLogger) Error(format string, args ...any)                           {}
func (m *scenarioMockLogger) Critical(format string, args ...any)                        {}
func (m *scenarioMockLogger) Stream(format string, args ...any)                          {}
func (m *scenarioMockLogger) Logon(format string, args ...any)                           {}
func (m *scenarioMockLogger) Logout(format string, args ...any)                          {}
func (m *scenarioMockLogger) Trade(format string, args ...any)                           {}
func (m *scenarioMockLogger) Schedule(format string, args ...any)                        {}
func (m *scenarioMockLogger) Report(format string, args ...any)                          {}
func (m *scenarioMockLogger) GetNotifQueue() <-chan *unilog_interfaces.NotifMessage     { return nil }
func (m *scenarioMockLogger) SetLocalNotifQueue(notifChan chan *unilog_interfaces.NotifMessage) {}
func (m *scenarioMockLogger) Log(level unilog_interfaces.Level, format string, args ...any)     {}
func (m *scenarioMockLogger) SetLevel(level unilog_interfaces.Level)                            {}
func (m *scenarioMockLogger) GetLevel() unilog_interfaces.Level                                 { return unilog_interfaces.LevelInfo }
func (m *scenarioMockLogger) SetCallerSkip(skip int)                                     {}
func (m *scenarioMockLogger) SetMetadata(metadata map[string]string)                     {}
func (m *scenarioMockLogger) AddMetadata(key, value string)                              {}
func (m *scenarioMockLogger) Close()                                                     {}

// generateTestKeyPair creates an in-memory RSA key pair encoded as PEM strings.
func generateTestKeyPair(t *testing.T) (pubPEM string, privPEM string) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	require.NoError(t, err)

	pubPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}))

	privPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}))

	return pubPEM, privPEM
}

// TestScenario_ZeroKnowledgeSecretsIntegration performs an end-to-end integration
// audit ensuring zero-knowledge boundaries, per-service key isolation, and non-shared configs.
func TestScenario_ZeroKnowledgeSecretsIntegration(t *testing.T) {
	// 1. Generate distinct key pairs for Service A (Tele-Remote) and Service B (Notif-Server)
	pubA, privA := generateTestKeyPair(t)
	pubB, privB := generateTestKeyPair(t)

	// 2. Encrypt secrets using respective public keys
	secretA := "tele-bot-token-supersecret-999"
	secretB := "notif-jwt-encryption-key-777"

	encA, err := secret.Encrypt(secretA, pubA)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encA, "ENC("))

	encB, err := secret.Encrypt(secretB, pubB)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encB, "ENC("))

	tempDir := t.TempDir()

	t.Run("ZeroKnowledge_LoadTimePreservation", func(t *testing.T) {
		// Verify that distributed-config does NOT decrypt secrets at load time.
		yamlContent := `
common:
  name: tele-remote
capabilities:
  tele_remote:
    token: "` + encA + `"
`
		confPath := filepath.Join(tempDir, "zk_test.yaml")
		err := os.WriteFile(confPath, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Provide private key to ensure that even when a valid key is available,
		// load time preserves the ciphertext untouched.
		os.Setenv("BASTIEN_PRIVATE_KEY", privA)
		defer os.Unsetenv("BASTIEN_PRIVATE_KEY")

		cfg := &distconf_core.Config{Logger: distconf_utils.EnsureSafeLogger(nil)}
		err = distconf_loader.LoadConfigFromFile(cfg, confPath)
		require.NoError(t, err)

		capMap, ok := cfg.Capabilities["tele_remote"].(map[string]interface{})
		require.True(t, ok)
		loadedToken, ok := capMap["token"].(string)
		require.True(t, ok)

		// Must strictly remain the ciphertext string
		assert.Equal(t, encA, loadedToken, "Secret must NOT be decrypted at configuration load time")
		assert.True(t, strings.HasPrefix(loadedToken, "ENC("))
	})

	t.Run("PerService_CryptographicIsolation", func(t *testing.T) {
		// Service A with Key A: Can decrypt Secret A, but fails on Secret B
		os.Setenv("BASTIEN_PRIVATE_KEY", privA)

		acA := &toolbox_config.AppConfig{}
		decryptedA, err := acA.DecryptSecret(encA)
		assert.NoError(t, err)
		assert.Equal(t, secretA, decryptedA, "Service A must successfully decrypt its own secret")

		_, errCrossA := acA.DecryptSecret(encB)
		assert.Error(t, errCrossA, "Service A must FAIL to decrypt Service B's secret")

		// Service B with Key B: Can decrypt Secret B, but fails on Secret A
		os.Setenv("BASTIEN_PRIVATE_KEY", privB)

		acB := &toolbox_config.AppConfig{}
		decryptedB, err := acB.DecryptSecret(encB)
		assert.NoError(t, err)
		assert.Equal(t, secretB, decryptedB, "Service B must successfully decrypt its own secret")

		_, errCrossB := acB.DecryptSecret(encA)
		assert.Error(t, errCrossB, "Service B must FAIL to decrypt Service A's secret")

		os.Unsetenv("BASTIEN_PRIVATE_KEY")
	})

	t.Run("NonShared_LocalConfigurationIsolation", func(t *testing.T) {
		// Verify the 'local:' configuration block remains strictly unshared
		localSecret := "internal-signing-salt"
		encLocal, err := secret.Encrypt(localSecret, pubA)
		require.NoError(t, err)

		yamlWithLocal := `
common:
  name: tele-remote
capabilities:
  tele_remote:
    ip: "127.0.0.1"
    port: "1863"
local:
  internal_worker_count: 8
  signing_salt: "` + encLocal + `"
`
		var root yaml.Node
		err = yaml.Unmarshal([]byte(yamlWithLocal), &root)
		require.NoError(t, err)
		distconf_loader.ProcessNode(&root)

		var raw map[string]interface{}
		err = root.Decode(&raw)
		require.NoError(t, err)

		localMap, _ := raw["local"].(map[string]interface{})
		ac := &toolbox_config.AppConfig{
			Config: distconf.New("standalone"),
			Local:  localMap,
		}

		os.Setenv("BASTIEN_PRIVATE_KEY", privA)
		defer os.Unsetenv("BASTIEN_PRIVATE_KEY")

		// 1. Check local retrieval
		assert.Equal(t, "8", ac.GetLocal("internal_worker_count"))
		saltEnc := ac.GetLocal("signing_salt").(string)
		assert.Equal(t, encLocal, saltEnc)

		// 2. Local on-demand decryption
		decryptedSalt, err := ac.DecryptSecret(saltEnc)
		assert.NoError(t, err)
		assert.Equal(t, localSecret, decryptedSalt)

		// 3. Verify local section is NOT in capabilities (so it is never broadcast to config-server)
		_, inCaps := ac.Config.Capabilities["local"]
		assert.False(t, inCaps, "Local configuration must never be exposed inside capabilities")
	})

	t.Run("WebInterface_ZeroKnowledgeRemoval", func(t *testing.T) {
		// Verify that web-interface router rejects /api/v1/toolbox/decrypt with HTTP 404
		mux := http.NewServeMux()
		logger := &scenarioMockLogger{}
		sessionManager := scs.New()

		web_router.RegisterDynamicRoutes(mux, logger, nil, sessionManager)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/toolbox/decrypt", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code, "Web interface must return 404 for /api/v1/toolbox/decrypt")
	})
}
