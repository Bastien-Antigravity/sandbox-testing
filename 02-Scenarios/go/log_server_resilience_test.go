package scenarios

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/Bastien-Antigravity/safe-socket/src/schemas"
	"github.com/stretchr/testify/assert"
)

func TestLogServerHardeningScenario(t *testing.T) {
	host := "localhost"
	tcpPort := "15000"

	// 1. Setup: Ensure log-server is reachable
	fmt.Println(">>> Starting Log Server Hardening Verification Scenario")
	
	t.Run("Handshake_Identity_Verification", func(t *testing.T) {
		fmt.Println(">>> Phase 1: Verifying Handshake Identity Extraction")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		if err == nil {
			defer conn.Close()
		}
		assert.NoError(t, err)
		if err != nil {
			return
		}

		// Build HelloMsg (Unpacked Cap'n Proto)
		msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
		assert.NoError(t, err)
		hello, err := schemas.NewRootHelloMsg(seg)
		assert.NoError(t, err)
		
		hello.SetFromName("HARDENING_TEST_SERVICE")
		hello.SetFromHost("TEST_CONTAINER")

		// Handshake is UNPACKED, so we send segment count then segment data
		bytes, err := msg.Marshal()
		assert.NoError(t, err)

		// Send length prefix (4 bytes BE) then payload
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(bytes)

		// Wait for server to process and log
		time.Sleep(2 * time.Second)

		// Verify identity in Docker logs
		logs := getDockerLogs("sandbox-log-server", 20)
		assert.Contains(t, logs, "client identified via handshake as 'HARDENING_TEST_SERVICE@TEST_CONTAINER'", 
			"Server should correctly identify the test service")
	})

	t.Run("Zombie_Pruning_60s_Timeout", func(t *testing.T) {
		fmt.Println(">>> Phase 2: Verifying 60s Zombie Pruning Timeout (this will take ~65 seconds)")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		if err == nil {
			defer conn.Close()
		}
		assert.NoError(t, err)
		if err != nil {
			return
		}

		// Perform handshake to avoid 5s handshake timeout
		msg, seg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
		hello, _ := schemas.NewRootHelloMsg(seg)
		hello.SetFromName("ZOMBIE_CANDIDATE")
		hello.SetFromHost("ZOMBIE_HOST")
		bytes, _ := msg.Marshal()
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(bytes)

		fmt.Println(">>> Handshake sent. Now remaining idle for 65 seconds...")
		
		// Wait for more than 60 seconds
		time.Sleep(65 * time.Second)

		// Verify pruning in Docker logs
		logs := getDockerLogs("sandbox-log-server", 50)
		assert.Contains(t, logs, "connection idle for 60s. Pruning zombie.", 
			"Server should have pruned the idle connection")
		
		// Verify socket is closed
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		one := make([]byte, 1)
		_, err = conn.Read(one)
		assert.Error(t, err, "Connection should have been closed by the server")
	})

	t.Run("Handshake_Timeout_5s", func(t *testing.T) {
		fmt.Println(">>> Phase 3: Verifying 5s Handshake Timeout (Slow-Loris protection)")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		if err == nil {
			defer conn.Close()
		}
		assert.NoError(t, err)
		if err != nil {
			return
		}

		// Send NOTHING. Wait for 7 seconds.
		time.Sleep(7 * time.Second)

		// Verify timeout in Docker logs
		logs := getDockerLogs("sandbox-log-server", 20)
		assert.Contains(t, logs, "handshake timeout from", "Server should log a handshake timeout")
		
		// Verify socket is closed
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		one := make([]byte, 1)
		_, err = conn.Read(one)
		assert.Error(t, err, "Server should have closed the connection due to handshake timeout")
	})
}
