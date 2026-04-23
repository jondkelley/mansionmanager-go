package instance

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
)

// CheckTCPListenPortsFree tries to bind each distinct port on all interfaces.
// If binding fails with “address already in use”, it returns an error suitable
// for surfacing when provisioning a new palace (something else holds the port).
func CheckTCPListenPortsFree(tcpPort, httpPort int) error {
	if tcpPort <= 0 || tcpPort > 65535 || httpPort <= 0 || httpPort > 65535 {
		return fmt.Errorf("invalid tcp/http port (%d / %d)", tcpPort, httpPort)
	}
	if err := tryListenOnce(tcpPort); err != nil {
		return err
	}
	if httpPort != tcpPort {
		if err := tryListenOnce(httpPort); err != nil {
			return err
		}
	}
	return nil
}

func tryListenOnce(port int) error {
	addr := net.JoinHostPort("", strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return fmt.Errorf("port %d is already in use on this server", port)
		}
		return fmt.Errorf("cannot verify port %d is free: %w", port, err)
	}
	_ = ln.Close()
	return nil
}
