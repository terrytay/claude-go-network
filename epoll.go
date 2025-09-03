package main

import (
	"fmt"
	"syscall"
)

// EpollEventLoop manages high-performance async I/O using Linux epoll
type EpollEventLoop struct {
	epollFd   int
	eventsFd  int
	maxEvents int
	events    []syscall.EpollEvent
	handlers  map[int]EventHandler
	running   bool
}

// EventHandler defines the interface for handling socket events
type EventHandler interface {
	OnRead(fd int) error
	OnWrite(fd int) error
	OnError(fd int, err error)
	OnClose(fd int)
}

// SocketEventHandler implements EventHandler for UDP sockets
type SocketEventHandler struct {
	socket    *LinuxUDPSocket
	onData    func(data []byte, from SocketAddr)
	onError   func(error)
	buffer    []byte
}

// NewEpollEventLoop creates a new epoll-based event loop
func NewEpollEventLoop(maxEvents int) (*EpollEventLoop, error) {
	// Create epoll instance
	epollFd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("failed to create epoll instance: %v", err)
	}

	return &EpollEventLoop{
		epollFd:   epollFd,
		maxEvents: maxEvents,
		events:    make([]syscall.EpollEvent, maxEvents),
		handlers:  make(map[int]EventHandler),
		running:   false,
	}, nil
}

// AddSocket adds a socket to the epoll event loop
func (el *EpollEventLoop) AddSocket(socket *LinuxUDPSocket, handler EventHandler) error {
	fd := socket.GetFD()
	
	// Set socket to non-blocking mode
	if err := socket.SetNonBlocking(true); err != nil {
		return fmt.Errorf("failed to set non-blocking: %v", err)
	}

	// Add socket to epoll with read events
	event := syscall.EpollEvent{
		Events: syscall.EPOLLIN | syscall.EPOLLET, // Edge-triggered for better performance
		Fd:     int32(fd),
	}

	if err := syscall.EpollCtl(el.epollFd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
		return fmt.Errorf("failed to add socket to epoll: %v", err)
	}

	// Store the handler
	el.handlers[fd] = handler

	return nil
}

// RemoveSocket removes a socket from the epoll event loop
func (el *EpollEventLoop) RemoveSocket(fd int) error {
	// Remove from epoll
	if err := syscall.EpollCtl(el.epollFd, syscall.EPOLL_CTL_DEL, fd, nil); err != nil {
		return fmt.Errorf("failed to remove socket from epoll: %v", err)
	}

	// Remove handler
	if handler, exists := el.handlers[fd]; exists {
		handler.OnClose(fd)
		delete(el.handlers, fd)
	}

	return nil
}

// Run starts the event loop (blocking)
func (el *EpollEventLoop) Run() error {
	el.running = true
	
	for el.running {
		// Wait for events with 1 second timeout
		n, err := syscall.EpollWait(el.epollFd, el.events, 1000)
		if err != nil {
			if err == syscall.EINTR {
				continue // Interrupted system call, continue
			}
			return fmt.Errorf("epoll_wait failed: %v", err)
		}

		// Process events
		for i := 0; i < n; i++ {
			event := el.events[i]
			fd := int(event.Fd)
			
			handler, exists := el.handlers[fd]
			if !exists {
				continue
			}

			// Handle different event types
			if event.Events&syscall.EPOLLIN != 0 {
				// Data available for reading
				if err := handler.OnRead(fd); err != nil {
					handler.OnError(fd, err)
				}
			}

			if event.Events&syscall.EPOLLOUT != 0 {
				// Socket ready for writing
				if err := handler.OnWrite(fd); err != nil {
					handler.OnError(fd, err)
				}
			}

			if event.Events&(syscall.EPOLLERR|syscall.EPOLLHUP) != 0 {
				// Error or hang-up occurred
				handler.OnError(fd, fmt.Errorf("socket error/hangup"))
			}
		}
	}

	return nil
}

// Stop stops the event loop
func (el *EpollEventLoop) Stop() {
	el.running = false
}

// Close cleans up the event loop
func (el *EpollEventLoop) Close() error {
	el.Stop()
	
	// Close all managed sockets
	for fd := range el.handlers {
		el.RemoveSocket(fd)
	}

	// Close epoll instance
	if el.epollFd > 0 {
		return syscall.Close(el.epollFd)
	}
	return nil
}

// GetStats returns event loop statistics
func (el *EpollEventLoop) GetStats() EventLoopStats {
	return EventLoopStats{
		ActiveConnections: len(el.handlers),
		MaxEvents:        el.maxEvents,
		Running:          el.running,
	}
}

// EventLoopStats holds statistics for the event loop
type EventLoopStats struct {
	ActiveConnections int
	MaxEvents        int
	Running          bool
}

// NewSocketEventHandler creates a new socket event handler
func NewSocketEventHandler(socket *LinuxUDPSocket, bufferSize int) *SocketEventHandler {
	return &SocketEventHandler{
		socket: socket,
		buffer: make([]byte, bufferSize),
	}
}

// SetDataCallback sets the callback for received data
func (h *SocketEventHandler) SetDataCallback(callback func(data []byte, from SocketAddr)) {
	h.onData = callback
}

// SetErrorCallback sets the callback for errors
func (h *SocketEventHandler) SetErrorCallback(callback func(error)) {
	h.onError = callback
}

// OnRead handles read events
func (h *SocketEventHandler) OnRead(fd int) error {
	for {
		n, fromAddr, err := h.socket.RecvFrom(h.buffer)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				// No more data available, normal for edge-triggered epoll
				break
			}
			return fmt.Errorf("recv error: %v", err)
		}

		if n > 0 && h.onData != nil {
			// Make a copy of the data for the callback
			data := make([]byte, n)
			copy(data, h.buffer[:n])
			h.onData(data, fromAddr)
		}
	}
	return nil
}

// OnWrite handles write events
func (h *SocketEventHandler) OnWrite(fd int) error {
	// For UDP, we typically don't need to handle write events
	// since UDP sends are usually non-blocking
	return nil
}

// OnError handles error events
func (h *SocketEventHandler) OnError(fd int, err error) {
	if h.onError != nil {
		h.onError(err)
	}
}

// OnClose handles close events
func (h *SocketEventHandler) OnClose(fd int) {
	// Cleanup if needed
}

// HighPerformanceServer demonstrates a high-performance UDP server using epoll
type HighPerformanceServer struct {
	socket     *LinuxUDPSocket
	eventLoop  *EpollEventLoop
	handler    *SocketEventHandler
	stats      ServerStats
}

// Note: ServerStats is defined in ultra_fast_server.go to avoid duplicate definition

// NewHighPerformanceServer creates a new high-performance UDP server
func NewHighPerformanceServer(bindIP string, bindPort uint16) (*HighPerformanceServer, error) {
	// Create socket
	socket, err := NewLinuxUDPSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %v", err)
	}

	// Bind to address
	if err := socket.Bind(bindIP, bindPort); err != nil {
		socket.Close()
		return nil, fmt.Errorf("failed to bind: %v", err)
	}

	// Create event loop
	eventLoop, err := NewEpollEventLoop(1000) // Handle up to 1000 concurrent events
	if err != nil {
		socket.Close()
		return nil, fmt.Errorf("failed to create event loop: %v", err)
	}

	// Create handler
	handler := NewSocketEventHandler(socket, 65536) // 64KB buffer

	server := &HighPerformanceServer{
		socket:    socket,
		eventLoop: eventLoop,
		handler:   handler,
	}

	// Set up callbacks
	handler.SetDataCallback(server.handleData)
	handler.SetErrorCallback(server.handleError)

	// Add socket to event loop
	if err := eventLoop.AddSocket(socket, handler); err != nil {
		socket.Close()
		eventLoop.Close()
		return nil, fmt.Errorf("failed to add socket to event loop: %v", err)
	}

	return server, nil
}

// handleData processes received data
func (s *HighPerformanceServer) handleData(data []byte, from SocketAddr) {
	s.stats.RequestsReceived++
	s.stats.BytesReceived += uint64(len(data))

	// Echo the data back (simple echo server)
	n, err := s.socket.SendTo(data, from.IP, from.Port)
	if err != nil {
		s.stats.Errors++
		return
	}

	s.stats.ResponsesSent++
	s.stats.BytesSent += uint64(n)
}

// handleError processes errors
func (s *HighPerformanceServer) handleError(err error) {
	s.stats.Errors++
}

// Run starts the server (blocking)
func (s *HighPerformanceServer) Run() error {
	return s.eventLoop.Run()
}

// Stop stops the server
func (s *HighPerformanceServer) Stop() {
	s.eventLoop.Stop()
}

// Close cleans up the server
func (s *HighPerformanceServer) Close() error {
	s.eventLoop.Close()
	return s.socket.Close()
}

// GetStats returns server statistics
func (s *HighPerformanceServer) GetStats() ServerStats {
	return s.stats
}

// GetAddress returns the server's bound address
func (s *HighPerformanceServer) GetAddress() SocketAddr {
	return s.socket.GetLocalAddr()
}

// ConnectionPool manages a pool of client connections for high throughput
type ConnectionPool struct {
	sockets   []*LinuxUDPSocket
	eventLoop *EpollEventLoop
	poolSize  int
	roundRobin int
}

// NewConnectionPool creates a connection pool for high-performance clients
func NewConnectionPool(poolSize int) (*ConnectionPool, error) {
	eventLoop, err := NewEpollEventLoop(poolSize * 2)
	if err != nil {
		return nil, fmt.Errorf("failed to create event loop: %v", err)
	}

	pool := &ConnectionPool{
		sockets:   make([]*LinuxUDPSocket, poolSize),
		eventLoop: eventLoop,
		poolSize:  poolSize,
	}

	// Create pool of sockets
	for i := 0; i < poolSize; i++ {
		socket, err := NewLinuxUDPSocket()
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create socket %d: %v", i, err)
		}
		pool.sockets[i] = socket

		// Add to event loop with a simple handler
		handler := NewSocketEventHandler(socket, 65536)
		if err := eventLoop.AddSocket(socket, handler); err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to add socket %d to event loop: %v", i, err)
		}
	}

	return pool, nil
}

// GetSocket returns the next socket in round-robin fashion
func (cp *ConnectionPool) GetSocket() *LinuxUDPSocket {
	socket := cp.sockets[cp.roundRobin]
	cp.roundRobin = (cp.roundRobin + 1) % cp.poolSize
	return socket
}

// Close closes all sockets in the pool
func (cp *ConnectionPool) Close() error {
	if cp.eventLoop != nil {
		cp.eventLoop.Close()
	}

	for _, socket := range cp.sockets {
		if socket != nil {
			socket.Close()
		}
	}

	return nil
}