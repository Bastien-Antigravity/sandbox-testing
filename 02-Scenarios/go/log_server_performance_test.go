package scenarios

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/Bastien-Antigravity/flexible-logger/src/schemas/capnp/logger"
	"github.com/Bastien-Antigravity/safe-socket/src/schemas"
	"github.com/stretchr/testify/assert"
)

func TestLogServerPerformanceScenario(t *testing.T) {
	host := "localhost"
	tcpPort := "15000"

	fmt.Println(">>> Starting Log Server Performance Verification Scenario")

	t.Run("High_Volume_Ingestion", func(t *testing.T) {
		fmt.Println(">>> Verifying High-Volume Ingestion (Async Console Worker)")
		
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		assert.NoError(t, err)
		defer conn.Close()

		// 1. Mandatory Handshake
		msg, seg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
		hello, _ := schemas.NewRootHelloMsg(seg)
		hello.SetFromName("PERF_STRESS_TESTER")
		hello.SetFromHost("LOAD_GENERATOR")
		bytes, _ := msg.Marshal()
		
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(bytes)

		fmt.Println(">>> Handshake sent. Starting burst of 5000 messages...")

		// 2. Burst of 5000 messages
		start := time.Now()
		count := 5000
		
		for i := 0; i < count; i++ {
			lMsg, lSeg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
			log, _ := logger.NewRootLoggerMsg(lSeg)
			log.SetMessage(fmt.Sprintf("STRESS_TEST_MESSAGE_%d", i))
			log.SetLoggerName("perf-tester")
			log.SetLevel(logger.Level_info)
			log.SetTimestamp(time.Now().Format(time.RFC3339Nano))
			
			lBytes, _ := lMsg.MarshalPacked() // packed as required by log-server
			
			binary.BigEndian.PutUint32(lenBuf, uint32(len(lBytes)))
			_, err = conn.Write(lenBuf)
			if err != nil {
				t.Fatalf("Failed to write length at message %d: %v", i, err)
			}
			_, err = conn.Write(lBytes)
			if err != nil {
				t.Fatalf("Failed to write payload at message %d: %v", i, err)
			}
		}

		duration := time.Since(start)
		fmt.Printf(">>> Ingested %d messages in %v (Avg: %v/msg)\n", count, duration, duration/time.Duration(count))

		// 3. Verification
		// We verify the ingestion didn't stall and was efficient.
		assert.True(t, duration < 10*time.Second, "Ingestion should be very fast with async console worker")
		
		fmt.Println(">>> High-Volume Ingestion Phase Completed")
	})
}
