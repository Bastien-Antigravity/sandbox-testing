package scenarios

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	config_schema "github.com/Bastien-Antigravity/distributed-config/src/schemas"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestConfigSyncOnArrival(t *testing.T) {
	host := "127.0.0.1"
	port := "3306"

	fmt.Println(">>> Starting Sync-on-Arrival Consistency Scenario")

	// 1. Client A: Connect and PUSH data
	fmt.Println(">>> Phase 1: Client A pushing data...")
	connA, err := net.Dial("tcp", host+":"+port)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	doHandshake(connA, "CLIENT_A")

	testVal := fmt.Sprintf("value_from_a_%d", time.Now().Unix())
	update := map[string]map[string]string{
		"sync_test": {"key_a": testVal},
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

	// Wait for ACK
	_ = connA.SetReadDeadline(time.Now().Add(2 * time.Second))
	respLenBuf := make([]byte, 4)
	_, err = io.ReadFull(connA, respLenBuf)
	assert.NoError(t, err)
	connA.Close()

	fmt.Println(">>> Client A disconnected. Waiting 2s for server processing...")
	time.Sleep(2 * time.Second)

	// 2. Client B: Connect LATER and verify it gets current state
	fmt.Println(">>> Phase 2: Client B connecting (Late Joiner)...")
	connB, err := net.Dial("tcp", host+":"+port)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	defer connB.Close()
	doHandshake(connB, "CLIENT_B")

	// Client B should request full sync immediately
	getMsg := &config_schema.ConfigMsg{
		Command: config_schema.ConfigMsg_GET_SYNC,
	}
	gBytes, _ := proto.Marshal(getMsg)
	// PROTOPITFALL: gBytes is empty because Command=0 is default.
	// safe-socket treats 0-length as heartbeat.
	// FIX: Manually append an unknown field [tag=15, wiretype=0 (varint), value=1]
	if len(gBytes) == 0 {
		gBytes = []byte{0x78, 0x01} 
	}
	
	binary.BigEndian.PutUint32(lenBuf, uint32(len(gBytes)))
	_, _ = connB.Write(lenBuf)
	_, _ = connB.Write(gBytes)

	// Read Loop
	var recoveredConfig map[string]map[string]string
	found := false
	for i := 0; i < 10; i++ {
		_ = connB.SetReadDeadline(time.Now().Add(5 * time.Second))
		respLenBuf2 := make([]byte, 4)
		_, err = io.ReadFull(connB, respLenBuf2)
		if err != nil {
			fmt.Printf(">>> Read length error (attempt %d): %v\n", i, err)
			break
		}
		respLen := binary.BigEndian.Uint32(respLenBuf2)
		fmt.Printf(">>> Received message with length: %d\n", respLen)
		
		if respLen == 0 { continue } // Heartbeat

		respData := make([]byte, respLen)
		_, err = io.ReadFull(connB, respData)
		assert.NoError(t, err)

		respMsg := &config_schema.ConfigMsg{}
		err = proto.Unmarshal(respData, respMsg)
		if err != nil { continue }

		if respMsg.Command == config_schema.ConfigMsg_GET_SYNC {
			err = json.Unmarshal(respMsg.Payload, &recoveredConfig)
			assert.NoError(t, err)
			found = true
			break
		}
	}

	assert.True(t, found, "Client B should have received the current state")
	assert.Equal(t, testVal, recoveredConfig["sync_test"]["key_a"], "Client B must see the value pushed by Client A")
	fmt.Printf(">>> SUCCESS: Client B recovered value: %s\n", recoveredConfig["sync_test"]["key_a"])
}
