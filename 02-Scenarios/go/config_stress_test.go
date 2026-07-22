package scenarios

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	config_schema "github.com/Bastien-Antigravity/distributed-config/src/schemas"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestConfigConcurrentStress(t *testing.T) {
	host := "127.0.0.1"
	port := "3306"
	numClients := 50

	fmt.Printf(">>> Starting Concurrent Stress Scenario with %d clients\n", numClients)

	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			defer wg.Done()
			
			conn, err := net.Dial("tcp", host+":"+port)
			if err != nil {
				t.Errorf("Client %d failed to connect: %v", id, err)
				return
			}
			defer conn.Close()

			doHandshake(conn, fmt.Sprintf("STRESS_CLIENT_%d", id))

			// Attempt to update the same key
			update := map[string]map[string]string{
				"stress_test": {"global_key": fmt.Sprintf("val_from_%d", id)},
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
			if err != nil {
				t.Errorf("Client %d failed to receive ACK: %v", id, err)
				return
			}
		}(i)
	}

	wg.Wait()
	fmt.Println(">>> All clients completed updates. Verifying server stability...")

	// Final verification: Server should still be alive and respond to GET_SYNC
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		t.Fatalf("Server died after stress test: %v", err)
	}
	defer conn.Close()
	doHandshake(conn, "STRESS_VERIFIER")

	getMsg := &config_schema.ConfigMsg{
		Command: config_schema.ConfigMsg_GET_SYNC,
	}
	gBytes, _ := proto.Marshal(getMsg)
	if len(gBytes) == 0 { gBytes = []byte{0x78, 0x01} } // Protopitfall fix
	
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(gBytes)))
	_, _ = conn.Write(lenBuf)
	_, _ = conn.Write(gBytes)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	respLenBuf := make([]byte, 4)
	_, err = io.ReadFull(conn, respLenBuf)
	assert.NoError(t, err, "Server should respond after stress")
	
	fmt.Println(">>> SUCCESS: Server survived high concurrency collision.")
}
