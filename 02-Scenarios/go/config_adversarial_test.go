package scenarios

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/Bastien-Antigravity/safe-socket/src/schemas"
	"github.com/stretchr/testify/assert"
)

func TestConfigServerAdversarialHardening(t *testing.T) {
	host := "127.0.0.1"
	port := "3306"

	t.Run("OversizedFrameHeader", func(t *testing.T) {
		fmt.Println(">>> Toxic Case: Sending oversized frame header (2GB)...")
		conn, err := net.Dial("tcp", host+":"+port)
		if err == nil {
			defer conn.Close()
		}
		assert.NoError(t, err)
		if err != nil {
			return
		}

		// Send handshake first (mandatory)
		doHandshake(conn, "TOXIC_OVERSIZE")

		// Send a length prefix that is way too large (MaxPayloadSize is 64MB)
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, 2*1024*1024*1024) // 2GB
		_, err = conn.Write(lenBuf)
		assert.NoError(t, err)

		// The server should close the connection because 2GB > MaxPayloadSize
		// Wait a bit for server to process
		time.Sleep(1 * time.Second)

		// Attempt to read - should fail or be EOF
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		buf := make([]byte, 1)
		_, err = conn.Read(buf)
		assert.Error(t, err, "Server should have closed connection for oversized frame")
		fmt.Println(">>> Result: Connection closed as expected.")
	})

	t.Run("MalformedProtobuf", func(t *testing.T) {
		fmt.Println(">>> Toxic Case: Sending malformed Protobuf data...")
		conn, err := net.Dial("tcp", host+":"+port)
		if err == nil {
			defer conn.Close()
		}
		assert.NoError(t, err)
		if err != nil {
			return
		}

		doHandshake(conn, "TOXIC_MALFORMED")

		// Send 10 bytes of garbage
		garbage := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(garbage)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(garbage)

		// Server should log error and close connection (or continue if it handles per-message errors)
		time.Sleep(1 * time.Second)

		// In config-server, ProcessRequest failure returns error which causes handleConnection to return, closing socket.
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 4)
		_, err = io.ReadFull(conn, buf)
		assert.Error(t, err, "Server should have closed connection for malformed protobuf (EOF expected)")
		fmt.Printf(">>> Result: Connection closed as expected (%v).\n", err)
	})

	t.Run("HandshakeOversizeName", func(t *testing.T) {
		fmt.Println(">>> Toxic Case: Handshake with 1MB name string...")
		conn, err := net.Dial("tcp", host+":"+port)
		if err == nil {
			defer conn.Close()
		}
		assert.NoError(t, err)
		if err != nil {
			return
		}

		// Construct massive handshake
		msg, seg, _ := capnp.NewMessage(capnp.SingleSegment(nil))
		hello, _ := schemas.NewRootHelloMsg(seg)

		hugeName := make([]byte, 1*1024*1024)
		for i := range hugeName {
			hugeName[i] = 'A'
		}
		hello.SetFromName(string(hugeName))
		hello.SetFromHost("LOAD_GEN")

		bytes, _ := msg.Marshal()
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(bytes)))
		_, _ = conn.Write(lenBuf)
		_, _ = conn.Write(bytes)

		time.Sleep(1 * time.Second)

		// The server should either reject it or handle it without crashing.
		// If it accepts it, it should still be alive.
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		buf := make([]byte, 1)
		_, err = conn.Read(buf)
		// We don't strictly care if it closes or stays open, as long as it doesn't crash.
		// But usually it should close if it's considered toxic.
		fmt.Printf(">>> Result: Server status after huge handshake: %v\n", err)
	})

	fmt.Println(">>> Adversarial Hardening Scenarios Completed.")
}
