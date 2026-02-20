package common

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

func GetEnvAsInt(name string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.Atoi(valueStr); err == nil && value >= 0 {
			return value
		}
	}
	return defaultValue
}

func GetEnvAsDuration(name string, defaultValue time.Duration) time.Duration {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := time.ParseDuration(valueStr); err == nil && value >= 0 {
			return value
		}
	}
	return defaultValue
}

func Drain[T any](ch <-chan T) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func writeUDPFrame(w net.Conn, data []byte) error {
	length := len(data)
	if length > 65535 {
		return fmt.Errorf("writeUDPFrame: datagram too large: %d", length)
	}
	header := [2]byte{byte(length >> 8), byte(length)}
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readUDPFrame(conn net.Conn, buf []byte, timeout time.Duration) (int, error) {
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}
	var header [2]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, err
	}
	length := int(header[0])<<8 | int(header[1])
	if length == 0 {
		return 0, nil
	}
	if length > len(buf) {
		return 0, fmt.Errorf("readUDPFrame: datagram too large: %d > buffer %d", length, len(buf))
	}
	if _, err := io.ReadFull(conn, buf[:length]); err != nil {
		return 0, err
	}
	return length, nil
}
