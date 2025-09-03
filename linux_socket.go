package main

import (
	"fmt"
	"syscall"
)

// LinuxUDPSocket represents a high-performance Linux UDP socket
type LinuxUDPSocket struct {
	fd          int
	localAddr   SocketAddr
	nonBlocking bool
}

// SocketAddr represents an IP address and port
type SocketAddr struct {
	IP   string
	Port uint16
}

// NewLinuxUDPSocket creates a new Linux UDP socket optimized for performance
func NewLinuxUDPSocket() (*LinuxUDPSocket, error) {
	// Create UDP socket with optimizations
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %v", err)
	}

	socket := &LinuxUDPSocket{
		fd: fd,
	}

	// Set socket options for better performance
	if err := socket.setSocketOptions(); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to set socket options: %v", err)
	}

	return socket, nil
}

// setSocketOptions configures the socket for high performance
func (s *LinuxUDPSocket) setSocketOptions() error {
	// Enable address reuse
	if err := syscall.SetsockoptInt(s.fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return fmt.Errorf("SO_REUSEADDR: %v", err)
	}

	// Enable port reuse (Linux specific)
	if err := syscall.SetsockoptInt(s.fd, syscall.SOL_SOCKET, unix_SO_REUSEPORT, 1); err != nil {
		// Not critical if this fails on older kernels
	}

	// Increase receive buffer size for high throughput
	if err := syscall.SetsockoptInt(s.fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 2*1024*1024); err != nil {
		return fmt.Errorf("SO_RCVBUF: %v", err)
	}

	// Increase send buffer size
	if err := syscall.SetsockoptInt(s.fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 2*1024*1024); err != nil {
		return fmt.Errorf("SO_SNDBUF: %v", err)
	}

	// Enable timestamp reception for precise RTT measurements
	if err := syscall.SetsockoptInt(s.fd, syscall.SOL_SOCKET, unix_SO_TIMESTAMPING,
		unix_SOF_TIMESTAMPING_RX_SOFTWARE|unix_SOF_TIMESTAMPING_TX_SOFTWARE); err != nil {
		// Not critical if this fails
	}

	return nil
}

// GetFD returns the socket file descriptor
func (s *LinuxUDPSocket) GetFD() int {
	return s.fd
}

// Bind binds the socket to a local address and port
func (s *LinuxUDPSocket) Bind(ip string, port uint16) error {
	ipBytes := parseIPv4(ip)
	if ipBytes == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	addr := syscall.SockaddrInet4{
		Port: int(port),
		Addr: [4]byte{ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3]},
	}

	if err := syscall.Bind(s.fd, &addr); err != nil {
		return fmt.Errorf("failed to bind socket: %v", err)
	}

	// Get the actual bound address
	boundAddr, err := syscall.Getsockname(s.fd)
	if err != nil {
		return fmt.Errorf("failed to get bound address: %v", err)
	}

	if boundInet4, ok := boundAddr.(*syscall.SockaddrInet4); ok {
		s.localAddr = SocketAddr{
			IP: fmt.Sprintf("%d.%d.%d.%d",
				boundInet4.Addr[0], boundInet4.Addr[1],
				boundInet4.Addr[2], boundInet4.Addr[3]),
			Port: uint16(boundInet4.Port),
		}
	}

	return nil
}

// GetLocalAddr returns the local address
func (s *LinuxUDPSocket) GetLocalAddr() SocketAddr {
	return s.localAddr
}

// SendTo sends data to a specific address
func (s *LinuxUDPSocket) SendTo(data []byte, ip string, port uint16) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	ipBytes := parseIPv4(ip)
	if ipBytes == nil {
		return 0, fmt.Errorf("invalid IP address: %s", ip)
	}

	destAddr := &syscall.SockaddrInet4{
		Port: int(port),
		Addr: [4]byte{ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3]},
	}

	err := syscall.Sendto(s.fd, data, 0, destAddr)
	if err != nil {
		return 0, fmt.Errorf("sendto failed: %v", err)
	}
	return len(data), nil
}

// RecvFrom receives data and returns sender address
func (s *LinuxUDPSocket) RecvFrom(buffer []byte) (int, SocketAddr, error) {
	n, from, err := syscall.Recvfrom(s.fd, buffer, 0)
	if err != nil {
		return 0, SocketAddr{}, fmt.Errorf("failed to receive: %v", err)
	}

	var fromAddr SocketAddr
	if fromInet4, ok := from.(*syscall.SockaddrInet4); ok {
		fromAddr = SocketAddr{
			IP: fmt.Sprintf("%d.%d.%d.%d",
				fromInet4.Addr[0], fromInet4.Addr[1],
				fromInet4.Addr[2], fromInet4.Addr[3]),
			Port: uint16(fromInet4.Port),
		}
	}

	return n, fromAddr, nil
}

// SetNonBlocking sets non-blocking mode
func (s *LinuxUDPSocket) SetNonBlocking(nonBlocking bool) error {
	// Use direct syscall for Linux compatibility
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(s.fd), syscall.F_GETFL, 0)
	if errno != 0 {
		return fmt.Errorf("failed to get socket flags: %v", errno)
	}

	if nonBlocking {
		flags |= syscall.O_NONBLOCK
	} else {
		flags &^= syscall.O_NONBLOCK
	}

	_, _, errno = syscall.Syscall(syscall.SYS_FCNTL, uintptr(s.fd), syscall.F_SETFL, flags)
	if errno != 0 {
		return fmt.Errorf("failed to set non-blocking mode: %v", errno)
	}

	s.nonBlocking = nonBlocking
	return nil
}

// IsNonBlocking returns whether socket is in non-blocking mode
func (s *LinuxUDPSocket) IsNonBlocking() bool {
	return s.nonBlocking
}

// Close closes the socket
func (s *LinuxUDPSocket) Close() error {
	if s.fd > 0 {
		err := syscall.Close(s.fd)
		s.fd = -1
		return err
	}
	return nil
}

// Linux-specific constants
const (
	unix_SO_REUSEPORT                 = 15
	unix_SO_TIMESTAMPING              = 37
	unix_SOF_TIMESTAMPING_RX_SOFTWARE = 1 << 0
	unix_SOF_TIMESTAMPING_TX_SOFTWARE = 1 << 1
)

// parseIPv4 converts IP string to byte array
func parseIPv4(ip string) []byte {
	var result [4]byte
	var octet int
	var octetIndex int

	for i := 0; i < len(ip); i++ {
		c := ip[i]
		if c >= '0' && c <= '9' {
			octet = octet*10 + int(c-'0')
			if octet > 255 {
				return nil
			}
		} else if c == '.' {
			if octetIndex >= 3 {
				return nil
			}
			result[octetIndex] = byte(octet)
			octet = 0
			octetIndex++
		} else {
			return nil
		}
	}

	if octetIndex != 3 {
		return nil
	}
	result[octetIndex] = byte(octet)
	return result[:]
}

// Network byte order conversion functions
func htons(host uint16) uint16 {
	return (host<<8)&0xff00 | (host>>8)&0x00ff
}

func ntohs(network uint16) uint16 {
	return (network<<8)&0xff00 | (network>>8)&0x00ff
}

func htonl(host uint32) uint32 {
	return ((host & 0x000000ff) << 24) |
		((host & 0x0000ff00) << 8) |
		((host & 0x00ff0000) >> 8) |
		((host & 0xff000000) >> 24)
}

func ntohl(network uint32) uint32 {
	return ((network & 0x000000ff) << 24) |
		((network & 0x0000ff00) << 8) |
		((network & 0x00ff0000) >> 8) |
		((network & 0xff000000) >> 24)
}
