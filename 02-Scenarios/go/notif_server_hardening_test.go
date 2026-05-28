package scenarios

import (
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/Bastien-Antigravity/notif-server/src/core"
	"github.com/Bastien-Antigravity/safe-socket/src/schemas"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
	"github.com/stretchr/testify/assert"
)

func TestNotifServerHardeningScenario(t *testing.T) {
	host := "localhost"
	tcpPort := "15002" 

	fmt.Println(">>> Starting Notif Server Hardening Verification Scenario")

	t.Run("Stable_Identity_Verification", func(t *testing.T) {
		fmt.Println(">>> Phase 1: Verifying Stable Identity (Port Stripping)")
		
		port := getDockerPort("sandbox-notif-server", "1026/tcp")
		if port != "" {
			tcpPort = port
		}

		conn, err := net.Dial("tcp", host+":"+tcpPort)
		if err != nil {
			t.Skipf("Notif Server not reachable on %s:%s, skipping", host, tcpPort)
			return
		}
		defer conn.Close()

		doHandshake(conn, "NOTIF_IDENTITY_TEST")
		time.Sleep(1 * time.Second)

		logs := getDockerLogs("sandbox-notif-server", 20)
		assert.Contains(t, logs, "Client identified: NOTIF_IDENTITY_TEST-", "Should identify via stable name")
	})
t.Run("Worker_Pool_Non_Blocking", func(t *testing.T) {
	fmt.Println(">>> Phase 2: Verifying Worker Pool Non-Blocking Ingestion")

	port := getDockerPort("sandbox-notif-server", "1026/tcp")
	if port == "" { port = tcpPort }

	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil { t.Skip("Notif Server not reachable"); return }
	defer conn.Close()

	doHandshake(conn, "BURST_TESTER")

	// 2. Send 5000 messages rapidly
	burstCount := 5000
	fmt.Printf(">>> Sending %d notifications rapidly...\n", burstCount)
	handler := notifier.NewNotifHandler("test", nil)
	start := time.Now()

	for i := 0; i < burstCount; i++ {
		nMsg := &utils.NotifMessage{
			Message: fmt.Sprintf("Alert %d", i),
			Tags:    []string{"DISCORD"}, 
		}
		bytes := handler.NotifNcapSerialize(nMsg)
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(bytes)
	}

	duration := time.Since(start)
	fmt.Printf(">>> Ingested %d notifications in %v (Avg: %v/msg)\n", burstCount, duration, duration/time.Duration(burstCount))

	// Verification: Ingestion should remain highly responsive (under 1s for 5000 msgs is safe)
	assert.True(t, duration < 1*time.Second, "Ingestion should be non-blocking and fast")
})
}


func doHandshake(conn net.Conn, name string) {
	msg, seg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
	hello, _ := schemas.NewRootHelloMsg(seg)
	hello.SetFromName(name)
	hello.SetFromHost("TEST_NODE")
	bytes, _ := msg.Marshal()
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
	_, _ = conn.Write(lenBuf)
	_, _ = conn.Write(bytes)
}

func getDockerPort(containerName, internalPort string) string {
	cmd := exec.Command("docker", "port", containerName, internalPort)
	out, err := cmd.CombinedOutput()
	if err != nil { return "" }
	parts := strings.Split(string(out), ":")
	if len(parts) < 2 { return "" }
	return strings.TrimSpace(parts[len(parts)-1])
}
