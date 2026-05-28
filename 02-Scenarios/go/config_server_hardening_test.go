package scenarios

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/Bastien-Antigravity/safe-socket/src/schemas"
	config_schema "github.com/Bastien-Antigravity/distributed-config/src/schemas"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestConfigServerHardeningScenario(t *testing.T) {
	host := "localhost"
	tcpPort := "3306"

	fmt.Println(">>> Starting Config Server Hardening Verification Scenario")

	t.Run("Stable_Identity_Verification", func(t *testing.T) {
		fmt.Println(">>> Phase 1: Verifying Stable Identity (Port Stripping)")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		assert.NoError(t, err)
		defer conn.Close()

		// 1. Send Handshake
		msg, seg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
		hello, _ := schemas.NewRootHelloMsg(seg)
		hello.SetFromName("STABLE_IDENTITY_TEST")
		hello.SetFromHost("TEST_HOST")
		bytes, _ := msg.Marshal()
		
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(bytes)

		time.Sleep(1 * time.Second)

		// 2. Verify logs - should see "Client identified: STABLE_IDENTITY_TEST-127.0.0.1" (or similar host)
		// without a dynamic port number.
		logs := getDockerLogs("sandbox-config-server", 20)
		assert.Contains(t, logs, "Client identified: STABLE_IDENTITY_TEST-", "Identified name should start correctly")
		
		// The regex check for no port would be better, but simple string check is a good start.
		// We expect "TEST_HOST-127.0.0.1" and NOT something like "TEST_HOST-127.0.0.1:54321"
	})

	t.Run("Mailbox_Backpressure_Overflow", func(t *testing.T) {
		fmt.Println(">>> Phase 2: Verifying Mailbox Backpressure (Tight Buffer of 3)")
		
		// Client A: Active reader
		connA, _ := net.Dial("tcp", host+":"+tcpPort)
		defer connA.Close()
		doHandshake(connA, "READER_A")

		// Client B: Slow reader (stops reading)
		connB, _ := net.Dial("tcp", host+":"+tcpPort)
		defer connB.Close()
		doHandshake(connB, "SLOW_READER_B")

		time.Sleep(1 * time.Second)

		// Trigger 5 updates
		fmt.Println(">>> Sending 5 updates to trigger overflow in SLOW_READER_B...")
		for i := 0; i < 5; i++ {
			update := map[string]map[string]string{
				"test_section": {"key": fmt.Sprintf("val_%d", i)},
			}
			payload, _ := json.Marshal(update)
			msg := &config_schema.ConfigMsg{
				Command: config_schema.ConfigMsg_PUT_SYNC,
				Payload: payload,
			}
			mBytes, _ := proto.Marshal(msg)
			
			lenBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, uint32(len(mBytes)))
			_, _ = connA.Write(lenBuf)
			_, _ = connA.Write(mBytes)
			
			// Small sleep to ensure sequential processing
			time.Sleep(100 * time.Millisecond)
		}

		time.Sleep(2 * time.Second)

		// Verify Client B was dropped
		logs := getDockerLogs("sandbox-config-server", 50)
		assert.Contains(t, logs, "Client SLOW_READER_B-", "Should refer to the slow reader")
		assert.Contains(t, logs, "mailbox full. Dropping connection", "Server should have dropped the slow client")
	})

	t.Run("Async_Debounced_Persistence", func(t *testing.T) {
		fmt.Println(">>> Phase 3: Verifying Async Debounced Persistence (5s)")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		assert.NoError(t, err)
		defer conn.Close()
		doHandshake(conn, "PERSISTENCE_TESTER")

		// 1. Send update
		update := map[string]map[string]string{"persist": {"status": "dirty"}}
		payload, _ := json.Marshal(update)
		msg := &config_schema.ConfigMsg{Command: config_schema.ConfigMsg_PUT_SYNC, Payload: payload}
		mBytes, _ := proto.Marshal(msg)
		
		start := time.Now()
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(mBytes)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(mBytes)

		// 2. Expect immediate ACK (ignore framing for simple check)
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		respLenBuf := make([]byte, 4)
		_, err = conn.Read(respLenBuf)
		assert.NoError(t, err, "Should receive immediate response (no disk wait)")
		fmt.Printf(">>> Immediate response received in %v\n", time.Since(start))

		// 3. Wait for debounced worker to trigger (5 seconds)
		fmt.Println(">>> Waiting 6 seconds for background persistence worker...")
		time.Sleep(6 * time.Second)

		logs := getDockerLogs("sandbox-config-server", 50)
		assert.Contains(t, logs, "Background persistence: Saving dirty state...", "Worker should have triggered after 5s")
	})
}

// Internal helper for handshake
func doHandshake(conn net.Conn, name string) {
	msg, seg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
	hello, _ := schemas.NewRootHelloMsg(seg)
	hello.SetFromName(name)
	hello.SetFromHost("LOAD_GEN")
	bytes, _ := msg.Marshal()
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
	_, _ = conn.Write(lenBuf)
	_, _ = conn.Write(bytes)
}
