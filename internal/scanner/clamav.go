package scanner

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	// clamavChunkSize is the chunk size for the INSTREAM protocol (64KB).
	clamavChunkSize = 64 * 1024

	// clamavDialTimeout is the timeout for connecting to clamd.
	clamavDialTimeout = 5 * time.Second

	// clamavReadTimeout is the timeout for reading the scan verdict.
	clamavReadTimeout = 30 * time.Second
)

// ClamAVScanner implements ExternalScanner using the ClamAV clamd daemon.
// It communicates via the INSTREAM protocol over TCP or Unix socket.
//
// The INSTREAM protocol:
//  1. Send "zINSTREAM\0"
//  2. Send data in chunks: 4-byte big-endian length + data
//  3. Send 4 zero bytes to signal end
//  4. Read response: "stream: OK\0" or "stream: <virus> FOUND\0"
type ClamAVScanner struct {
	network string // "tcp" or "unix"
	address string // "127.0.0.1:3310" or "/run/clamav/clamd.ctl"
}

// NewClamAVScanner creates a ClamAV scanner from an address string.
// Address format: "tcp://host:port" or "unix:/path/to/socket"
func NewClamAVScanner(addr string) (*ClamAVScanner, error) {
	network, address, err := parseClamAVAddress(addr)
	if err != nil {
		return nil, err
	}
	return &ClamAVScanner{network: network, address: address}, nil
}

func (c *ClamAVScanner) Name() string {
	return "clamav"
}

// ScanFile sends a single file to clamd via INSTREAM and returns findings.
func (c *ClamAVScanner) ScanFile(ctx context.Context, filePath string, data []byte) ([]Finding, error) {
	result, err := c.scanINSTREAM(ctx, data)
	if err != nil {
		return nil, err
	}

	// ClamAV returns "stream: OK" for clean files.
	if strings.Contains(result, "OK") {
		return nil, nil
	}

	// ClamAV returns "stream: <virus> FOUND" for detections.
	if strings.Contains(result, "FOUND") {
		virusName := extractVirusName(result)
		return []Finding{{
			Stage:       stageNameExternal,
			Severity:    SeverityBlock,
			Category:    "malware_detected",
			FilePath:    filePath,
			Description: fmt.Sprintf("ClamAV detected malware: %s", virusName),
		}}, nil
	}

	// ClamAV returns "stream: <something> ERROR" on scan errors.
	if strings.Contains(result, "ERROR") {
		return nil, fmt.Errorf("clamd scan error: %s", result)
	}

	return nil, fmt.Errorf("unexpected clamd response: %s", result)
}

// scanINSTREAM sends data to clamd via the INSTREAM protocol.
func (c *ClamAVScanner) scanINSTREAM(ctx context.Context, data []byte) (string, error) {
	// Connect to clamd.
	dialer := net.Dialer{Timeout: clamavDialTimeout}
	conn, err := dialer.DialContext(ctx, c.network, c.address)
	if err != nil {
		return "", fmt.Errorf("connect to clamd at %s://%s: %w", c.network, c.address, err)
	}
	defer conn.Close() //nolint:errcheck

	// Send INSTREAM command (null-terminated).
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return "", fmt.Errorf("send INSTREAM command: %w", err)
	}

	// Send data in chunks.
	reader := bytes.NewReader(data)
	chunk := make([]byte, clamavChunkSize)
	for {
		n, err := reader.Read(chunk)
		if n > 0 {
			// 4-byte big-endian length header.
			var lenBuf [4]byte
			binary.BigEndian.PutUint32(lenBuf[:], uint32(n))
			if _, werr := conn.Write(lenBuf[:]); werr != nil {
				return "", fmt.Errorf("send chunk length: %w", werr)
			}
			if _, werr := conn.Write(chunk[:n]); werr != nil {
				return "", fmt.Errorf("send chunk data: %w", werr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read data: %w", err)
		}
	}

	// Send 4 zero bytes to signal end of stream.
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return "", fmt.Errorf("send end of stream: %w", err)
	}

	// Read response.
	if err := conn.SetReadDeadline(time.Now().Add(clamavReadTimeout)); err != nil {
		return "", fmt.Errorf("set read deadline: %w", err)
	}

	var resp bytes.Buffer
	if _, err := io.Copy(&resp, io.LimitReader(conn, 4096)); err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	result := strings.TrimRight(resp.String(), "\x00\n\r ")
	return result, nil
}

// parseClamAVAddress parses "tcp://host:port" or "unix:/path" into network + address.
func parseClamAVAddress(addr string) (string, string, error) {
	if strings.HasPrefix(addr, "tcp://") {
		return "tcp", strings.TrimPrefix(addr, "tcp://"), nil
	}
	if strings.HasPrefix(addr, "unix:") {
		return "unix", strings.TrimPrefix(addr, "unix:"), nil
	}
	// Default to TCP if no scheme.
	if strings.Contains(addr, ":") {
		return "tcp", addr, nil
	}
	return "", "", fmt.Errorf("invalid ClamAV address %q: expected tcp://host:port or unix:/path", addr)
}

// extractVirusName extracts the virus name from a clamd response like "stream: Eicar-Signature FOUND".
func extractVirusName(response string) string {
	// Response format: "stream: <virus name> FOUND"
	response = strings.TrimPrefix(response, "stream: ")
	response = strings.TrimSuffix(response, " FOUND")
	return strings.TrimSpace(response)
}

// readAllLimited reads from r up to maxBytes. Returns error if limit exceeded.
func readAllLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("data exceeds %d bytes", maxBytes)
	}
	return data, nil
}
