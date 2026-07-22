package scenarios

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	config_schema "github.com/Bastien-Antigravity/distributed-config/src/schemas"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestConfigServerPersistenceRecovery(t *testing.T) {
	host := "127.0.0.1"
	tcpPort := "3306"
	storePath := "config_persistence_test.json"

	fmt.Println(">>> Starting Config Server Persistence Recovery Scenario")
	
	// 0. Clean start
	_ = exec.Command("pkill", "-9", "-x", "config-server").Run()
	_ = os.Remove(storePath)
	time.Sleep(2 * time.Second)

	fmt.Println(">>> Starting initial config-server...")
	initialCmd := exec.Command("../../../config-server/bin/config-server", "--store", storePath)
	if err := initialCmd.Start(); err != nil {
		t.Fatalf("Failed to start initial config-server: %v", err)
	}
	time.Sleep(5 * time.Second)

	// 1. Connect and Push Data
	fmt.Println(">>> Phase 1: Pushing data to Config Server...")
	
	var conn net.Conn
	var err error
	for i := 0; i < 15; i++ {
		conn, err = net.Dial("tcp", host+":"+tcpPort)
		if err == nil {
			break
		}
		fmt.Printf(">>> Waiting for config-server to be ready... (%v)\n", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		t.Fatalf("Failed to connect to config-server: %v", err)
	}
	
	doHandshake(conn, "PERSISTENCE_WRITER")

	testSection := "persistence_test"
	testKey := "survivor_key"
	testVal := fmt.Sprintf("val_%d", time.Now().Unix())

	update := map[string]map[string]string{
		testSection: {testKey: testVal},
	}
	payload, _ := json.Marshal(update)
	msg := &config_schema.ConfigMsg{
		Command: config_schema.ConfigMsg_PUT_SYNC,
		Payload: payload,
	}
	mBytes, _ := proto.Marshal(msg)
	
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(mBytes)))
	_, _ = conn.Write(lenBuf)
	_, _ = conn.Write(mBytes)

	// Wait for ACK
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	respLenBuf := make([]byte, 4)
	_, err = io.ReadFull(conn, respLenBuf)
	assert.NoError(t, err, "Should receive ACK for PUT")
	conn.Close()

	// 2. Wait for background persistence (debounced 5s)
	fmt.Println(">>> Waiting 7 seconds for background persistence to trigger...")
	time.Sleep(7 * time.Second)

	// 3. Restart the Server (Cold Boot)
	fmt.Println(">>> Phase 2: Restarting config-server (Cold Boot)...")
	
	// Aggressive Kill
	_ = exec.Command("pkill", "-9", "-x", "config-server").Run()
	time.Sleep(3 * time.Second)

	// Verify death
	for i := 0; i < 5; i++ {
		c, err := net.DialTimeout("tcp", host+":"+tcpPort, 500*time.Millisecond)
		if err != nil {
			fmt.Println(">>> Config-server confirmed dead.")
			break
		}
		c.Close()
		fmt.Println(">>> Config-server still alive, killing harder...")
		_ = exec.Command("pkill", "-9", "-x", "config-server").Run()
		time.Sleep(1 * time.Second)
	}

	// Start it again in background with explicit store
	goCmd := exec.Command("../../../config-server/bin/config-server", "--store", storePath)
	err = goCmd.Start()
	if err != nil {
		t.Fatalf("Failed to restart config-server natively: %v", err)
	}
	// Give it a moment to bind
	time.Sleep(5 * time.Second)

	// 4. Connect again and Verify Data
	fmt.Println(">>> Phase 3: Verifying data recovery...")
	var conn2 net.Conn
	for i := 0; i < 15; i++ {
		conn2, err = net.Dial("tcp", host+":"+tcpPort)
		if err == nil {
			break
		}
		fmt.Printf(">>> Waiting for rebooted config-server... (%v)\n", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		t.Fatalf("Failed to connect to config-server after restart: %v", err)
	}
	defer conn2.Close()
	doHandshake(conn2, "PERSISTENCE_VERIFIER")

	getMsg := &config_schema.ConfigMsg{
		Command: config_schema.ConfigMsg_GET_SYNC,
	}
	gBytes, _ := proto.Marshal(getMsg)
	// FIX: Ensure non-empty message for safe-socket heartbeat compatibility
	if len(gBytes) == 0 {
		gBytes = []byte{0x78, 0x01}
	}
	
	binary.BigEndian.PutUint32(lenBuf, uint32(len(gBytes)))
	_, _ = conn2.Write(lenBuf)
	_, _ = conn2.Write(gBytes)

	// Read Response (Loop until we get GET_SYNC because we might get BROADCASTs)
	var recoveredConfig map[string]map[string]string
	found := false
	for i := 0; i < 20; i++ {
		_ = conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		respLenBuf2 := make([]byte, 4)
		_, err = io.ReadFull(conn2, respLenBuf2)
		if err != nil {
			fmt.Printf(">>> Read length error (attempt %d): %v\n", i, err)
			if err == io.EOF {
				fmt.Println(">>> Connection CLOSED by server.")
				break
			}
			continue
		}
		respLen := binary.BigEndian.Uint32(respLenBuf2)
		fmt.Printf(">>> Received message with length: %d\n", respLen)
		
		if respLen == 0 {
			fmt.Println(">>> Received HEARTBEAT (length 0)")
			continue
		}

		respData := make([]byte, respLen)
		_, err = io.ReadFull(conn2, respData)
		if err != nil {
			t.Fatalf("Failed to read response data: %v", err)
		}

		respMsg := &config_schema.ConfigMsg{}
		err = proto.Unmarshal(respData, respMsg)
		if err != nil {
			fmt.Printf(">>> FAILED to unmarshal message (first 10 bytes: %x): %v\n", respData[:10], err)
			continue
		}

		fmt.Printf(">>> Received command: %v\n", respMsg.Command)
		if respMsg.Command == config_schema.ConfigMsg_GET_SYNC {
			err = json.Unmarshal(respMsg.Payload, &recoveredConfig)
			if err != nil {
				t.Fatalf("Failed to unmarshal config payload: %v (Payload: %s)", err, string(respMsg.Payload))
			}
			found = true
			break
		}
		fmt.Printf(">>> Still waiting for GET_SYNC...\n")
	}

	if !found {
		t.Fatal("Failed to receive GET_SYNC response after 20 attempts")
	}

	assert.Equal(t, testVal, recoveredConfig[testSection][testKey], "Value should have survived the restart")
	fmt.Printf(">>> SUCCESS: Recovered value: %s\n", recoveredConfig[testSection][testKey])
}
