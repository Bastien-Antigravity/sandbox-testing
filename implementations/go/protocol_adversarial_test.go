package scenarios

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProtocolHardeningAdversarial(t *testing.T) {
	host := "localhost"
	tcpPort := "15000"

	t.Run("Timeout_Protection", func(t *testing.T) {
		fmt.Println(">>> QA Test: Timeout Protection (Slow-Loris)")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		assert.NoError(t, err)
		defer conn.Close()

		// Send NOTHING. Wait for 6 seconds.
		// The server should close the connection after 5 seconds.
		start := time.Now()
		buf := make([]byte, 1)
		_ = conn.SetReadDeadline(time.Now().Add(7 * time.Second))
		n, err := conn.Read(buf)
		
		duration := time.Since(start)
		fmt.Printf(">>> Connection closed after %v (Read: %d, Error: %v)\n", duration, n, err)

		assert.Error(t, err, "Server should have closed the connection")
		assert.True(t, duration >= 5*time.Second, "Timeout should be at least 5 seconds")
		assert.True(t, duration < 7*time.Second, "Timeout should trigger before 7 seconds")
	})

	t.Run("OOM_Protection", func(t *testing.T) {
		fmt.Println(">>> QA Test: OOM Protection (Message Too Large)")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		assert.NoError(t, err)
		defer conn.Close()

		// Send a length prefix of 11MB (over the 10MB limit)
		msgLen := uint32(11 * 1024 * 1024)
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, msgLen)
		
		_, err = conn.Write(lenBuf)
		assert.NoError(t, err)

		// The server should immediately close the connection upon receiving the invalid length
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 1)
		_, err = conn.Read(buf)
		
		assert.Error(t, err, "Server should have rejected the 11MB prefix and closed the connection")
	})

	t.Run("Handshake_Bypass_Protection", func(t *testing.T) {
		fmt.Println(">>> QA Test: Handshake Bypass Protection")
		conn, err := net.Dial("tcp", host+":"+tcpPort)
		assert.NoError(t, err)
		defer conn.Close()

		// Send a length prefix of 10 bytes, but NOT starting with 0,0,0,0 (not a Cap'n Proto unpacked message)
		// Or send valid data but NOT a handshake.
		msgLen := uint32(10)
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, msgLen)
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write([]byte("RANDOMDATA"))

		// Server should reject it
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 1)
		_, err = conn.Read(buf)
		assert.Error(t, err, "Server should have rejected data that skipped the handshake")
	})
}
