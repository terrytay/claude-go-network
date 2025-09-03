package main

import (
	"testing"
	"time"
)

// Test reliability layer functionality
func TestReliabilityLayer(t *testing.T) {
	// Test sequence number management
	t.Run("SequenceNumberManagement", func(t *testing.T) {
		rel := NewReliabilityLayer()
		
		// Test initial sequence number
		if rel.NextSeqNum() != 1 {
			t.Errorf("Expected initial sequence number 1, got %d", rel.NextSeqNum())
		}
		
		// Test sequence number increment
		seq1 := rel.GetNextSeqNum()
		seq2 := rel.GetNextSeqNum()
		if seq2 != seq1+1 {
			t.Errorf("Expected sequence numbers to increment: %d -> %d", seq1, seq2)
		}
	})
	
	// Test acknowledgment handling
	t.Run("AcknowledgmentHandling", func(t *testing.T) {
		rel := NewReliabilityLayer()
		
		// Send a packet and expect it to be stored for potential retransmission
		packet := NewPacket(DATA_PACKET, 0, 100, 0, []byte("test data"))
		rel.SendPacket(packet)
		
		// Verify packet is in unacked list
		if !rel.HasUnackedPacket(100) {
			t.Error("Packet should be in unacked list after sending")
		}
		
		// Acknowledge the packet
		ackPacket := NewPacket(ACK_PACKET, ACK_FLAG, 0, 101, nil)
		rel.HandleAck(ackPacket)
		
		// Verify packet is removed from unacked list
		if rel.HasUnackedPacket(100) {
			t.Error("Packet should be removed from unacked list after ACK")
		}
	})
	
	// Test retransmission timeout
	t.Run("RetransmissionTimeout", func(t *testing.T) {
		rel := NewReliabilityLayer()
		rel.SetRetransmissionTimeout(50 * time.Millisecond)
		
		// Send a packet
		packet := NewPacket(DATA_PACKET, 0, 200, 0, []byte("timeout test"))
		rel.SendPacket(packet)
		
		// Wait for timeout
		time.Sleep(60 * time.Millisecond)
		
		// Check if packet needs retransmission
		timedOut := rel.GetTimedOutPackets()
		if len(timedOut) != 1 {
			t.Errorf("Expected 1 timed out packet, got %d", len(timedOut))
		}
		
		if timedOut[0].SeqNum != 200 {
			t.Errorf("Expected timed out packet seq 200, got %d", timedOut[0].SeqNum)
		}
	})
	
	// Test duplicate detection
	t.Run("DuplicateDetection", func(t *testing.T) {
		rel := NewReliabilityLayer()
		
		// Receive a packet
		packet1 := NewPacket(DATA_PACKET, 0, 300, 0, []byte("duplicate test"))
		isDuplicate1 := rel.IsPacketDuplicate(packet1)
		rel.MarkPacketReceived(packet1)
		
		if isDuplicate1 {
			t.Error("First packet should not be marked as duplicate")
		}
		
		// Receive the same packet again
		packet2 := NewPacket(DATA_PACKET, 0, 300, 0, []byte("duplicate test"))
		isDuplicate2 := rel.IsPacketDuplicate(packet2)
		
		if !isDuplicate2 {
			t.Error("Duplicate packet should be detected")
		}
	})
	
	// Test flow control
	t.Run("FlowControl", func(t *testing.T) {
		rel := NewReliabilityLayer()
		rel.SetWindowSize(3) // Small window for testing
		
		// Send packets up to window size
		for i := 0; i < 3; i++ {
			packet := NewPacket(DATA_PACKET, 0, uint32(400+i), 0, []byte("flow control test"))
			canSend := rel.CanSendPacket()
			if !canSend {
				t.Errorf("Should be able to send packet %d within window", i)
			}
			rel.SendPacket(packet)
		}
		
		// Try to send beyond window size
		canSend := rel.CanSendPacket()
		if canSend {
			t.Error("Should not be able to send beyond window size")
		}
		
		// ACK one packet to free up window space
		ackPacket := NewPacket(ACK_PACKET, ACK_FLAG, 0, 401, nil)
		rel.HandleAck(ackPacket)
		
		// Should now be able to send again
		canSend = rel.CanSendPacket()
		if !canSend {
			t.Error("Should be able to send after ACK frees window space")
		}
	})
}

// Test packet ordering
func TestPacketOrdering(t *testing.T) {
	rel := NewReliabilityLayer()
	
	// Receive packets out of order
	packet3 := NewPacket(DATA_PACKET, 0, 503, 0, []byte("packet 3"))
	packet1 := NewPacket(DATA_PACKET, 0, 501, 0, []byte("packet 1"))
	packet2 := NewPacket(DATA_PACKET, 0, 502, 0, []byte("packet 2"))
	
	// Process packets out of order
	rel.ReceivePacket(packet3)
	rel.ReceivePacket(packet1)
	rel.ReceivePacket(packet2)
	
	// Get ordered packets
	orderedPackets := rel.GetOrderedPackets()
	
	if len(orderedPackets) != 3 {
		t.Errorf("Expected 3 ordered packets, got %d", len(orderedPackets))
	}
	
	// Verify order
	expectedSeqs := []uint32{501, 502, 503}
	for i, packet := range orderedPackets {
		if packet.SeqNum != expectedSeqs[i] {
			t.Errorf("Packet %d: expected seq %d, got %d", 
				i, expectedSeqs[i], packet.SeqNum)
		}
	}
}

// Test congestion control simulation
func TestCongestionControl(t *testing.T) {
	rel := NewReliabilityLayer()
	
	// Test congestion window initialization
	cwnd := rel.GetCongestionWindow()
	if cwnd != 1 {
		t.Errorf("Expected initial congestion window 1, got %d", cwnd)
	}
	
	// Simulate successful transmission (slow start)
	for i := 0; i < 5; i++ {
		packet := NewPacket(DATA_PACKET, 0, uint32(600+i), 0, []byte("congestion test"))
		rel.SendPacket(packet)
		
		// Simulate ACK
		ackPacket := NewPacket(ACK_PACKET, ACK_FLAG, 0, uint32(601+i), nil)
		rel.HandleAck(ackPacket)
	}
	
	// Congestion window should have grown (slow start phase)
	cwnd = rel.GetCongestionWindow()
	if cwnd <= 1 {
		t.Errorf("Expected congestion window to grow, but got %d", cwnd)
	}
	
	// Simulate packet loss (timeout)
	rel.SimulatePacketLoss()
	
	// Congestion window should be reduced
	newCwnd := rel.GetCongestionWindow()
	if newCwnd >= cwnd {
		t.Errorf("Expected congestion window to reduce after loss: %d -> %d", cwnd, newCwnd)
	}
}

// Test RTT measurement
func TestRTTMeasurement(t *testing.T) {
	rel := NewReliabilityLayer()
	
	// Send a packet and measure time
	packet := NewPacket(DATA_PACKET, 0, 700, 0, []byte("RTT test"))
	sendTime := time.Now()
	rel.SendPacketWithTimestamp(packet, sendTime)
	
	// Simulate network delay
	time.Sleep(10 * time.Millisecond)
	
	// Receive ACK
	ackPacket := NewPacket(ACK_PACKET, ACK_FLAG, 0, 701, nil)
	rel.HandleAck(ackPacket)
	
	// Get RTT measurement
	rtt := rel.GetAverageRTT()
	if rtt <= 0 {
		t.Error("Expected positive RTT measurement")
	}
	
	if rtt < 5*time.Millisecond || rtt > 50*time.Millisecond {
		t.Errorf("Expected RTT around 10ms, got %v", rtt)
	}
}

// Test error handling
func TestReliabilityErrorHandling(t *testing.T) {
	rel := NewReliabilityLayer()
	
	// Test handling invalid ACK numbers
	t.Run("InvalidACK", func(t *testing.T) {
		// Send packet with seq 800
		packet := NewPacket(DATA_PACKET, 0, 800, 0, []byte("error test"))
		rel.SendPacket(packet)
		
		// Try to ACK with invalid number
		invalidAck := NewPacket(ACK_PACKET, ACK_FLAG, 0, 900, nil) // Way ahead
		err := rel.HandleAck(invalidAck)
		
		if err == nil {
			t.Error("Expected error for invalid ACK number")
		}
	})
	
	// Test buffer overflow protection
	t.Run("BufferOverflow", func(t *testing.T) {
		rel.SetMaxBufferSize(2) // Small buffer
		
		// Try to receive more packets than buffer can handle
		for i := 0; i < 5; i++ {
			packet := NewPacket(DATA_PACKET, 0, uint32(900+i), 0, []byte("overflow test"))
			err := rel.ReceivePacket(packet)
			
			if i >= 2 && err == nil {
				t.Errorf("Expected buffer overflow error at packet %d", i)
			}
		}
	})
}