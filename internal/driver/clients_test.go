package driver

import (
	"bufio"
	"io"
	"net"
	"testing"
)

func TestOpenVPNRevocationWaitsForCommandResponse(t *testing.T) {
	for _, response := range []string{"SUCCESS: common name removed", "ERROR: common name 'alice' not found"} {
		t.Run(response, func(t *testing.T) {
			client, connection := net.Pipe()
			done := make(chan string, 1)
			go func() {
				defer connection.Close()
				scanner := bufio.NewScanner(connection)
				if scanner.Scan() == false {
					done <- "missing kill command"
					return
				}
				command := scanner.Text()
				_, _ = io.WriteString(connection, ">INFO:OpenVPN Management Interface\r\n"+response+"\r\n")
				scanner.Scan()
				done <- command
			}()
			result := disconnectOpenVPNSession(client, "alice")
			if len(result) != 1 || result[0].OK == false {
				t.Fatalf("failed revocation: %+v", result)
			}
			if command := <-done; command != "kill alice" {
				t.Fatalf("unexpected command %q", command)
			}
		})
	}
}
