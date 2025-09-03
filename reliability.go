package main

import (
	"fmt"
	"sync"
	"time"
)

// ReliabilityLayer handles packet reliability, ordering, and congestion control
type ReliabilityLayer struct {
	// Sequence number management
	nextSeqNum uint32
	seqMutex   sync.Mutex
	
	// Unacknowledged packets for retransmission
	unackedPackets map[uint32]*UnackedPacket
	unackedMutex   sync.RWMutex
	
	// Received packets for duplicate detection and ordering
	receivedSeqs   map[uint32]bool
	receivedMutex  sync.RWMutex
	
	// Packet ordering buffer
	orderingBuffer map[uint32]*Packet
	orderingMutex  sync.RWMutex
	nextExpectedSeq uint32
	
	// Flow control
	windowSize     uint32
	windowMutex    sync.RWMutex
	
	// Congestion control
	congestionWindow uint32
	ssthresh        uint32 // Slow start threshold
	congestionMutex sync.RWMutex
	
	// RTT measurement
	rttSamples    []time.Duration
	rttMutex      sync.RWMutex
	averageRTT    time.Duration
	
	// Configuration
	retransmissionTimeout time.Duration
	maxBufferSize        int
}

// UnackedPacket stores packet with timestamp for retransmission
type UnackedPacket struct {
	Packet    *Packet
	SentTime  time.Time
	RetryCount int
}

// NewReliabilityLayer creates a new reliability layer
func NewReliabilityLayer() *ReliabilityLayer {
	return &ReliabilityLayer{
		nextSeqNum:            1,
		unackedPackets:       make(map[uint32]*UnackedPacket),
		receivedSeqs:         make(map[uint32]bool),
		orderingBuffer:       make(map[uint32]*Packet),
		nextExpectedSeq:      1,
		windowSize:           32, // Default window size
		congestionWindow:     1,  // Start with 1 (slow start)
		ssthresh:            32, // Initial slow start threshold
		retransmissionTimeout: 1000 * time.Millisecond,
		maxBufferSize:        1000,
		rttSamples:           make([]time.Duration, 0),
		averageRTT:           100 * time.Millisecond, // Initial estimate
	}
}

// Sequence number management
func (r *ReliabilityLayer) NextSeqNum() uint32 {
	r.seqMutex.Lock()
	defer r.seqMutex.Unlock()
	return r.nextSeqNum
}

func (r *ReliabilityLayer) GetNextSeqNum() uint32 {
	r.seqMutex.Lock()
	defer r.seqMutex.Unlock()
	seq := r.nextSeqNum
	r.nextSeqNum++
	return seq
}

// Packet sending with reliability tracking
func (r *ReliabilityLayer) SendPacket(packet *Packet) {
	r.SendPacketWithTimestamp(packet, time.Now())
}

func (r *ReliabilityLayer) SendPacketWithTimestamp(packet *Packet, timestamp time.Time) {
	if packet.IsDataPacket() {
		r.unackedMutex.Lock()
		r.unackedPackets[packet.SeqNum] = &UnackedPacket{
			Packet:     packet,
			SentTime:   timestamp,
			RetryCount: 0,
		}
		r.unackedMutex.Unlock()
	}
}

// Check if packet is unacknowledged
func (r *ReliabilityLayer) HasUnackedPacket(seqNum uint32) bool {
	r.unackedMutex.RLock()
	defer r.unackedMutex.RUnlock()
	_, exists := r.unackedPackets[seqNum]
	return exists
}

// Acknowledgment handling
func (r *ReliabilityLayer) HandleAck(ackPacket *Packet) error {
	if !ackPacket.HasAck() {
		return fmt.Errorf("packet is not an acknowledgment")
	}
	
	ackNum := ackPacket.AckNum
	
	r.unackedMutex.Lock()
	defer r.unackedMutex.Unlock()
	
	// Find corresponding unacked packet (ACK number - 1)
	seqNum := ackNum - 1
	unackedPacket, exists := r.unackedPackets[seqNum]
	
	if !exists {
		// This might be a duplicate ACK or invalid ACK
		if seqNum > r.nextSeqNum {
			return fmt.Errorf("ACK for future packet: ack=%d, next_seq=%d", ackNum, r.nextSeqNum)
		}
		return nil // Ignore duplicate/old ACKs
	}
	
	// Calculate RTT and update measurements
	rtt := time.Since(unackedPacket.SentTime)
	r.updateRTT(rtt)
	
	// Remove from unacked packets
	delete(r.unackedPackets, seqNum)
	
	// Update congestion control
	r.handleSuccessfulAck()
	
	return nil
}

// Get packets that have timed out
func (r *ReliabilityLayer) GetTimedOutPackets() []*Packet {
	r.unackedMutex.RLock()
	defer r.unackedMutex.RUnlock()
	
	now := time.Now()
	var timedOut []*Packet
	
	for _, unackedPacket := range r.unackedPackets {
		if now.Sub(unackedPacket.SentTime) > r.retransmissionTimeout {
			timedOut = append(timedOut, unackedPacket.Packet)
		}
	}
	
	return timedOut
}

// Packet receiving and duplicate detection
func (r *ReliabilityLayer) IsPacketDuplicate(packet *Packet) bool {
	r.receivedMutex.RLock()
	defer r.receivedMutex.RUnlock()
	return r.receivedSeqs[packet.SeqNum]
}

func (r *ReliabilityLayer) MarkPacketReceived(packet *Packet) {
	r.receivedMutex.Lock()
	r.receivedSeqs[packet.SeqNum] = true
	r.receivedMutex.Unlock()
}

func (r *ReliabilityLayer) ReceivePacket(packet *Packet) error {
	// Check buffer size limit
	r.orderingMutex.RLock()
	bufferSize := len(r.orderingBuffer)
	r.orderingMutex.RUnlock()
	
	if bufferSize >= r.maxBufferSize {
		return fmt.Errorf("receive buffer overflow: size=%d, max=%d", bufferSize, r.maxBufferSize)
	}
	
	// Check for duplicates
	if r.IsPacketDuplicate(packet) {
		return nil // Ignore duplicates silently
	}
	
	// Mark as received
	r.MarkPacketReceived(packet)
	
	// Add to ordering buffer
	r.orderingMutex.Lock()
	r.orderingBuffer[packet.SeqNum] = packet
	r.orderingMutex.Unlock()
	
	return nil
}

// Get packets in order
func (r *ReliabilityLayer) GetOrderedPackets() []*Packet {
	r.orderingMutex.Lock()
	defer r.orderingMutex.Unlock()
	
	var orderedPackets []*Packet
	
	// Collect packets in sequence starting from nextExpectedSeq
	for {
		packet, exists := r.orderingBuffer[r.nextExpectedSeq]
		if !exists {
			break
		}
		
		orderedPackets = append(orderedPackets, packet)
		delete(r.orderingBuffer, r.nextExpectedSeq)
		r.nextExpectedSeq++
	}
	
	return orderedPackets
}

// Flow control
func (r *ReliabilityLayer) CanSendPacket() bool {
	r.unackedMutex.RLock()
	unackedCount := len(r.unackedPackets)
	r.unackedMutex.RUnlock()
	
	r.windowMutex.RLock()
	windowSize := r.windowSize
	r.windowMutex.RUnlock()
	
	return uint32(unackedCount) < windowSize
}

func (r *ReliabilityLayer) SetWindowSize(size uint32) {
	r.windowMutex.Lock()
	r.windowSize = size
	r.windowMutex.Unlock()
}

// Congestion control
func (r *ReliabilityLayer) GetCongestionWindow() uint32 {
	r.congestionMutex.RLock()
	defer r.congestionMutex.RUnlock()
	return r.congestionWindow
}

func (r *ReliabilityLayer) handleSuccessfulAck() {
	r.congestionMutex.Lock()
	defer r.congestionMutex.Unlock()
	
	if r.congestionWindow < r.ssthresh {
		// Slow start: exponential growth
		r.congestionWindow++
	} else {
		// Congestion avoidance: linear growth
		// Increase by 1/cwnd per ACK (approximated)
		if r.congestionWindow > 0 {
			r.congestionWindow += 1 / r.congestionWindow
		}
	}
}

func (r *ReliabilityLayer) SimulatePacketLoss() {
	r.congestionMutex.Lock()
	defer r.congestionMutex.Unlock()
	
	// Multiplicative decrease
	r.ssthresh = r.congestionWindow / 2
	if r.ssthresh < 1 {
		r.ssthresh = 1
	}
	r.congestionWindow = r.ssthresh
}

// RTT measurement
func (r *ReliabilityLayer) updateRTT(sample time.Duration) {
	r.rttMutex.Lock()
	defer r.rttMutex.Unlock()
	
	// Keep last 10 samples for moving average
	r.rttSamples = append(r.rttSamples, sample)
	if len(r.rttSamples) > 10 {
		r.rttSamples = r.rttSamples[1:]
	}
	
	// Calculate average
	var total time.Duration
	for _, rtt := range r.rttSamples {
		total += rtt
	}
	r.averageRTT = total / time.Duration(len(r.rttSamples))
	
	// Update retransmission timeout based on RTT
	// RTO = average_RTT * 2 (simplified)
	r.retransmissionTimeout = r.averageRTT * 2
	if r.retransmissionTimeout < 100*time.Millisecond {
		r.retransmissionTimeout = 100 * time.Millisecond
	}
	if r.retransmissionTimeout > 5*time.Second {
		r.retransmissionTimeout = 5 * time.Second
	}
}

func (r *ReliabilityLayer) GetAverageRTT() time.Duration {
	r.rttMutex.RLock()
	defer r.rttMutex.RUnlock()
	return r.averageRTT
}

// Configuration
func (r *ReliabilityLayer) SetRetransmissionTimeout(timeout time.Duration) {
	r.retransmissionTimeout = timeout
}

func (r *ReliabilityLayer) SetMaxBufferSize(size int) {
	r.maxBufferSize = size
}