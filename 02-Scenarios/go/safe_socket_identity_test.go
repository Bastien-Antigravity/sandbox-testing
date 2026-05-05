package scenarios

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSafeSocketIdentityHandshake(t *testing.T) {
	// 1. Setup: Ensure Docker CLI is accessible
	_, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("Docker CLI not found, skipping integration test")
	}

	fmt.Println(">>> Starting SafeSocket Identity Handshake Scenario")

	// 2. Wait for notif-server to connect to log-server
	fmt.Println(">>> Waiting for notif-server to identify itself to log-server...")
	time.Sleep(10 * time.Second)

	// 3. Check logs of sandbox-log-server for re-identification message
	logCmd := exec.Command("docker", "logs", "sandbox-log-server")
	var out bytes.Buffer
	logCmd.Stdout = &out
	_ = logCmd.Run()

	srvLogs := out.String()
	fmt.Printf(">>> Log-Server Logs:\n%s\n", srvLogs)

	// 4. Assert that the log-server identified a client via handshake
	// The hybrid listener logs: "client identified via handshake as '...'"
	assert.Contains(t, srvLogs, "client identified via handshake as", "Log server should have identified a client via handshake")
	
	// Specifically check for notif-server (which we just updated with new dependencies)
	assert.Contains(t, srvLogs, "notif-server@", "Log server should have identified the notif-server")

	fmt.Println(">>> SafeSocket Identity Handshake Scenario Completed")
}
