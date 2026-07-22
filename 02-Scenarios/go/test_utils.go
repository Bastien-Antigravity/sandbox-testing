package scenarios

import (
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"

	"capnproto.org/go/capnp/v3"
	"github.com/Bastien-Antigravity/safe-socket/src/schemas"
)

// doHandshake performs a standard tcp-hello handshake for testing purposes.
func doHandshake(conn net.Conn, name string) {
	if conn == nil {
		return
	}
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

// getDockerLogs fetches the last N lines of logs from a container.
func getDockerLogs(containerName string, lines int) string {
	cmd := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", lines), containerName)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// contains is a primitive string containment check.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && len(s) > 0 && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()))
}
