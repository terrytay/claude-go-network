package main

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// LockFreeReliabilityLayer implements reliability without mutex locks
type LockFreeReliabilityLayer struct {
	// Atomic sequence number management
	nextSeqNum uint64
	
	// Lock-free hash table for unacknowledged packets
	unackedTable  *LockFreeHashTable
	
	// Lock-free queue for received packets
	recvQueue     *LockFreeQueue
	
	// Lock-free circular buffer for packet ordering
	orderBuffer   *LockFreeRingBuffer
	
	// Atomic configuration values
	windowSize    uint32
	congWindow    uint32
	rttEstimate   uint64 // nanoseconds
	timeoutBase   uint64 // nanoseconds
	
	// Performance counters (atomic)
	packetsSent   uint64
	packetsRecv   uint64
	packetsLost   uint64
	packetsRetr   uint64
}

// NewLockFreeReliabilityLayer creates a new lock-free reliability layer
func NewLockFreeReliabilityLayer() *LockFreeReliabilityLayer {
	return &LockFreeReliabilityLayer{
		nextSeqNum:   1,
		unackedTable: NewLockFreeHashTable(16384), // 16K entries
		recvQueue:    NewLockFreeQueue(8192),      // 8K packet queue
		orderBuffer:  NewLockFreeRingBuffer(4096), // 4K ordering buffer
		windowSize:   32,
		congWindow:   1,
		rttEstimate:  uint64(100 * time.Millisecond), // 100ms initial RTT
		timeoutBase:  uint64(1000 * time.Millisecond), // 1s base timeout
	}
}

// GetNextSeqNum atomically gets the next sequence number
func (rf *LockFreeReliabilityLayer) GetNextSeqNum() uint32 {
	return uint32(atomic.AddUint64(&rf.nextSeqNum, 1) - 1)
}

// SendPacket records a packet for potential retransmission (lock-free)
func (rf *LockFreeReliabilityLayer) SendPacket(packet *Packet) bool {
	if !packet.IsDataPacket() {
		return true // Don't track non-data packets
	}

	now := uint64(time.Now().UnixNano())
	entry := &UnackedEntry{
		Packet:    packet,
		SendTime:  now,
		RetryCount: 0,
	}

	// Insert into lock-free hash table
	success := rf.unackedTable.Insert(uint64(packet.SeqNum), unsafe.Pointer(entry))
	if success {
		atomic.AddUint64(&rf.packetsSent, 1)
	}
	return success
}

// HandleAck processes acknowledgment (lock-free)
func (rf *LockFreeReliabilityLayer) HandleAck(ackPacket *Packet) bool {
	if !ackPacket.HasAck() {
		return false
	}

	seqNum := ackPacket.AckNum - 1 // ACK number is next expected sequence
	
	// Remove from unacked table
	entryPtr := rf.unackedTable.Remove(uint64(seqNum))
	if entryPtr == nil {
		return false // Already acked or invalid
	}

	entry := (*UnackedEntry)(entryPtr)
	
	// Calculate RTT and update estimate
	now := uint64(time.Now().UnixNano())
	rtt := now - entry.SendTime
	rf.updateRTTAtomic(rtt)
	
	// Update congestion window
	rf.updateCongestionWindow(true)
	
	return true
}

// ReceivePacket handles incoming packet (lock-free)
func (rf *LockFreeReliabilityLayer) ReceivePacket(packet *Packet) bool {
	if !packet.IsDataPacket() {
		return true // Don't queue non-data packets
	}

	// Check for duplicates using atomic operations
	if rf.isDuplicate(packet.SeqNum) {
		return false
	}

	// Add to receive queue
	success := rf.recvQueue.Enqueue(unsafe.Pointer(packet))
	if success {
		atomic.AddUint64(&rf.packetsRecv, 1)
		rf.markReceived(packet.SeqNum)
	}
	return success
}

// GetTimedOutPackets returns packets that need retransmission (lock-free scan)
func (rf *LockFreeReliabilityLayer) GetTimedOutPackets() []*Packet {
	now := uint64(time.Now().UnixNano())
	timeout := atomic.LoadUint64(&rf.timeoutBase)
	
	var timedOut []*Packet
	
	// Scan hash table for timed out packets
	rf.unackedTable.ForEach(func(key uint64, valuePtr unsafe.Pointer) bool {
		entry := (*UnackedEntry)(valuePtr)
		
		if now - entry.SendTime > timeout {
			timedOut = append(timedOut, entry.Packet)
			// Update retry count atomically
			atomic.AddUint32(&entry.RetryCount, 1)
			atomic.AddUint64(&rf.packetsRetr, 1)
			
			// Update send time for next timeout calculation
			atomic.StoreUint64(&entry.SendTime, now)
			
			// Handle congestion (packet loss detected)
			rf.updateCongestionWindow(false)
		}
		
		return true // Continue iteration
	})
	
	if len(timedOut) > 0 {
		atomic.AddUint64(&rf.packetsLost, uint64(len(timedOut)))
	}
	
	return timedOut
}

// GetOrderedPackets returns packets in sequence order (lock-free)
func (rf *LockFreeReliabilityLayer) GetOrderedPackets() []*Packet {
	var orderedPackets []*Packet
	
	// Dequeue packets from receive queue
	for {
		packetPtr := rf.recvQueue.Dequeue()
		if packetPtr == nil {
			break
		}
		packet := (*Packet)(packetPtr)
		orderedPackets = append(orderedPackets, packet)
	}
	
	// TODO: Implement proper ordering using the ring buffer
	// For now, return packets as received
	return orderedPackets
}

// updateRTTAtomic updates RTT estimate using atomic operations
func (rf *LockFreeReliabilityLayer) updateRTTAtomic(sampleRTT uint64) {
	// Exponential weighted moving average: RTT = 0.875 * RTT + 0.125 * sample
	for {
		oldRTT := atomic.LoadUint64(&rf.rttEstimate)
		newRTT := (oldRTT * 7 + sampleRTT) / 8
		
		if atomic.CompareAndSwapUint64(&rf.rttEstimate, oldRTT, newRTT) {
			// Update timeout based on new RTT estimate
			newTimeout := newRTT * 4 // RTO = 4 * RTT (simplified)
			if newTimeout < uint64(100 * time.Millisecond) {
				newTimeout = uint64(100 * time.Millisecond)
			}
			if newTimeout > uint64(5 * time.Second) {
				newTimeout = uint64(5 * time.Second)
			}
			atomic.StoreUint64(&rf.timeoutBase, newTimeout)
			break
		}
	}
}

// updateCongestionWindow updates congestion window atomically
func (rf *LockFreeReliabilityLayer) updateCongestionWindow(success bool) {
	if success {
		// Successful ACK - increase window (slow start or congestion avoidance)
		for {
			oldWindow := atomic.LoadUint32(&rf.congWindow)
			newWindow := oldWindow
			
			if oldWindow < atomic.LoadUint32(&rf.windowSize) / 2 {
				// Slow start: exponential growth
				newWindow = oldWindow + 1
			} else {
				// Congestion avoidance: linear growth
				newWindow = oldWindow + 1 / oldWindow
			}
			
			if newWindow > atomic.LoadUint32(&rf.windowSize) {
				newWindow = atomic.LoadUint32(&rf.windowSize)
			}
			
			if atomic.CompareAndSwapUint32(&rf.congWindow, oldWindow, newWindow) {
				break
			}
		}
	} else {
		// Packet loss detected - reduce window
		for {
			oldWindow := atomic.LoadUint32(&rf.congWindow)
			newWindow := oldWindow / 2
			if newWindow < 1 {
				newWindow = 1
			}
			
			if atomic.CompareAndSwapUint32(&rf.congWindow, oldWindow, newWindow) {
				break
			}
		}
	}
}

// isDuplicate checks if packet is duplicate (lock-free)
func (rf *LockFreeReliabilityLayer) isDuplicate(seqNum uint32) bool {
	// Simplified duplicate detection using a bloom filter-like approach
	// In a real implementation, you'd use a more sophisticated data structure
	return false // For now, assume no duplicates
}

// markReceived marks a packet as received (lock-free)
func (rf *LockFreeReliabilityLayer) markReceived(seqNum uint32) {
	// TODO: Implement lock-free received packet tracking
}

// GetStats returns performance statistics
func (rf *LockFreeReliabilityLayer) GetStats() ReliabilityStats {
	return ReliabilityStats{
		PacketsSent:        atomic.LoadUint64(&rf.packetsSent),
		PacketsReceived:    atomic.LoadUint64(&rf.packetsRecv),
		PacketsLost:        atomic.LoadUint64(&rf.packetsLost),
		PacketsRetransmitted: atomic.LoadUint64(&rf.packetsRetr),
		CongestionWindow:   atomic.LoadUint32(&rf.congWindow),
		WindowSize:         atomic.LoadUint32(&rf.windowSize),
		RTTEstimate:        time.Duration(atomic.LoadUint64(&rf.rttEstimate)),
		TimeoutValue:       time.Duration(atomic.LoadUint64(&rf.timeoutBase)),
	}
}

// ReliabilityStats holds reliability layer statistics
type ReliabilityStats struct {
	PacketsSent          uint64
	PacketsReceived      uint64
	PacketsLost          uint64
	PacketsRetransmitted uint64
	CongestionWindow     uint32
	WindowSize           uint32
	RTTEstimate          time.Duration
	TimeoutValue         time.Duration
}

// UnackedEntry represents an unacknowledged packet
type UnackedEntry struct {
	Packet     *Packet
	SendTime   uint64
	RetryCount uint32
}

// Lock-Free Data Structures

// LockFreeHashTable implements a lock-free hash table
type LockFreeHashTable struct {
	buckets []unsafe.Pointer
	size    uint64
	mask    uint64
}

// NewLockFreeHashTable creates a new lock-free hash table
func NewLockFreeHashTable(size uint64) *LockFreeHashTable {
	// Ensure size is power of 2
	if size&(size-1) != 0 {
		panic("Hash table size must be power of 2")
	}
	
	return &LockFreeHashTable{
		buckets: make([]unsafe.Pointer, size),
		size:    size,
		mask:    size - 1,
	}
}

// Insert inserts a key-value pair (returns false if key exists)
func (ht *LockFreeHashTable) Insert(key uint64, value unsafe.Pointer) bool {
	hash := key & ht.mask
	
	for {
		current := atomic.LoadPointer(&ht.buckets[hash])
		if current != nil {
			return false // Key already exists
		}
		
		if atomic.CompareAndSwapPointer(&ht.buckets[hash], nil, value) {
			return true
		}
	}
}

// Remove removes a key and returns the value
func (ht *LockFreeHashTable) Remove(key uint64) unsafe.Pointer {
	hash := key & ht.mask
	
	for {
		current := atomic.LoadPointer(&ht.buckets[hash])
		if current == nil {
			return nil // Key doesn't exist
		}
		
		if atomic.CompareAndSwapPointer(&ht.buckets[hash], current, nil) {
			return current
		}
	}
}

// ForEach iterates over all entries (not guaranteed to be consistent)
func (ht *LockFreeHashTable) ForEach(fn func(key uint64, value unsafe.Pointer) bool) {
	for i := uint64(0); i < ht.size; i++ {
		value := atomic.LoadPointer(&ht.buckets[i])
		if value != nil {
			if !fn(i, value) {
				break
			}
		}
	}
}

// LockFreeQueue implements a lock-free FIFO queue
type LockFreeQueue struct {
	head unsafe.Pointer
	tail unsafe.Pointer
}

// QueueNode represents a node in the lock-free queue
type QueueNode struct {
	next unsafe.Pointer
	data unsafe.Pointer
}

// NewLockFreeQueue creates a new lock-free queue
func NewLockFreeQueue(capacity int) *LockFreeQueue {
	dummy := &QueueNode{}
	return &LockFreeQueue{
		head: unsafe.Pointer(dummy),
		tail: unsafe.Pointer(dummy),
	}
}

// Enqueue adds an item to the queue
func (q *LockFreeQueue) Enqueue(data unsafe.Pointer) bool {
	newNode := &QueueNode{data: data}
	newNodePtr := unsafe.Pointer(newNode)
	
	for {
		tail := atomic.LoadPointer(&q.tail)
		tailNode := (*QueueNode)(tail)
		next := atomic.LoadPointer(&tailNode.next)
		
		if tail == atomic.LoadPointer(&q.tail) { // Consistency check
			if next == nil {
				// Try to link new node
				if atomic.CompareAndSwapPointer(&tailNode.next, nil, newNodePtr) {
					// Try to advance tail
					atomic.CompareAndSwapPointer(&q.tail, tail, newNodePtr)
					return true
				}
			} else {
				// Help advance tail
				atomic.CompareAndSwapPointer(&q.tail, tail, next)
			}
		}
	}
}

// Dequeue removes and returns an item from the queue
func (q *LockFreeQueue) Dequeue() unsafe.Pointer {
	for {
		head := atomic.LoadPointer(&q.head)
		tail := atomic.LoadPointer(&q.tail)
		headNode := (*QueueNode)(head)
		next := atomic.LoadPointer(&headNode.next)
		
		if head == atomic.LoadPointer(&q.head) { // Consistency check
			if head == tail {
				if next == nil {
					return nil // Queue is empty
				}
				// Help advance tail
				atomic.CompareAndSwapPointer(&q.tail, tail, next)
			} else {
				if next == nil {
					continue // Inconsistent state, retry
				}
				
				data := (*QueueNode)(next).data
				
				// Try to advance head
				if atomic.CompareAndSwapPointer(&q.head, head, next) {
					return data
				}
			}
		}
	}
}

// LockFreeRingBuffer implements a lock-free ring buffer for packet ordering
type LockFreeRingBuffer struct {
	buffer []unsafe.Pointer
	size   uint64
	mask   uint64
	head   uint64
	tail   uint64
}

// NewLockFreeRingBuffer creates a new lock-free ring buffer
func NewLockFreeRingBuffer(size uint64) *LockFreeRingBuffer {
	// Ensure size is power of 2
	if size&(size-1) != 0 {
		panic("Ring buffer size must be power of 2")
	}
	
	return &LockFreeRingBuffer{
		buffer: make([]unsafe.Pointer, size),
		size:   size,
		mask:   size - 1,
	}
}

// Put inserts an item at the specified index
func (rb *LockFreeRingBuffer) Put(index uint64, data unsafe.Pointer) bool {
	pos := index & rb.mask
	return atomic.CompareAndSwapPointer(&rb.buffer[pos], nil, data)
}

// Get retrieves an item at the specified index
func (rb *LockFreeRingBuffer) Get(index uint64) unsafe.Pointer {
	pos := index & rb.mask
	return atomic.LoadPointer(&rb.buffer[pos])
}

// Remove removes an item at the specified index
func (rb *LockFreeRingBuffer) Remove(index uint64) unsafe.Pointer {
	pos := index & rb.mask
	for {
		current := atomic.LoadPointer(&rb.buffer[pos])
		if current == nil {
			return nil
		}
		if atomic.CompareAndSwapPointer(&rb.buffer[pos], current, nil) {
			return current
		}
	}
}