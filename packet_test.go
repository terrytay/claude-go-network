package main

import (
	"bytes"
	"fmt"
	"testing"
)

// Test packet creation and basic properties
func TestNewPacket(t *testing.T) {
	testCases := []struct {
		name        string
		packetType  uint8
		flags       uint8
		seqNum      uint32
		ackNum      uint32
		payload     []byte
		expectedLen uint16
	}{
		{
			name:        "Data packet with small payload",
			packetType:  DATA_PACKET,
			flags:       0,
			seqNum:      1000,
			ackNum:      2000,
			payload:     []byte("Hello World"),
			expectedLen: PACKET_HEADER_SIZE + 11,
		},
		{
			name:        "ACK packet with no payload",
			packetType:  ACK_PACKET,
			flags:       ACK_FLAG,
			seqNum:      0,
			ackNum:      1001,
			payload:     nil,
			expectedLen: PACKET_HEADER_SIZE,
		},
		{
			name:        "SYN packet with flags",
			packetType:  SYN_PACKET,
			flags:       SYN_FLAG,
			seqNum:      0,
			ackNum:      0,
			payload:     []byte("connection request"),
			expectedLen: PACKET_HEADER_SIZE + 18,
		},
		{
			name:        "Data packet with maximum payload",
			packetType:  DATA_PACKET,
			flags:       0,
			seqNum:      5000,
			ackNum:      0,
			payload:     make([]byte, MAX_PAYLOAD_SIZE),
			expectedLen: PACKET_HEADER_SIZE + MAX_PAYLOAD_SIZE,
		},
		{
			name:        "Oversized payload should be truncated",
			packetType:  DATA_PACKET,
			flags:       0,
			seqNum:      6000,
			ackNum:      0,
			payload:     make([]byte, MAX_PAYLOAD_SIZE+500), // Oversized
			expectedLen: PACKET_HEADER_SIZE + MAX_PAYLOAD_SIZE, // Should be truncated
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			packet := NewPacket(tc.packetType, tc.flags, tc.seqNum, tc.ackNum, tc.payload)

			// Verify basic properties
			if packet.Version != PROTOCOL_VERSION {
				t.Errorf("Expected version %d, got %d", PROTOCOL_VERSION, packet.Version)
			}
			if packet.Type != tc.packetType {
				t.Errorf("Expected type %d, got %d", tc.packetType, packet.Type)
			}
			if packet.Flags != tc.flags {
				t.Errorf("Expected flags %d, got %d", tc.flags, packet.Flags)
			}
			if packet.SeqNum != tc.seqNum {
				t.Errorf("Expected seqNum %d, got %d", tc.seqNum, packet.SeqNum)
			}
			if packet.AckNum != tc.ackNum {
				t.Errorf("Expected ackNum %d, got %d", tc.ackNum, packet.AckNum)
			}
			if packet.Length != tc.expectedLen {
				t.Errorf("Expected length %d, got %d", tc.expectedLen, packet.Length)
			}

			// Verify payload handling
			expectedPayloadLen := int(tc.expectedLen) - PACKET_HEADER_SIZE
			if len(packet.Payload) != expectedPayloadLen {
				t.Errorf("Expected payload length %d, got %d", expectedPayloadLen, len(packet.Payload))
			}
		})
	}
}

// Test packet serialization and deserialization
func TestPacketSerializeDeserialize(t *testing.T) {
	testCases := []struct {
		name       string
		packet     *Packet
		shouldFail bool
	}{
		{
			name: "Simple data packet",
			packet: NewPacket(DATA_PACKET, 0, 1000, 2000, []byte("test data")),
			shouldFail: false,
		},
		{
			name: "ACK packet with flags",
			packet: NewPacket(ACK_PACKET, ACK_FLAG, 0, 1001, nil),
			shouldFail: false,
		},
		{
			name: "SYN+ACK packet",
			packet: NewPacket(SYN_PACKET, SYN_FLAG|ACK_FLAG, 100, 200, []byte("handshake")),
			shouldFail: false,
		},
		{
			name: "Large payload packet",
			packet: NewPacket(DATA_PACKET, 0, 5000, 6000, make([]byte, 1000)),
			shouldFail: false,
		},
		{
			name: "Empty payload packet",
			packet: NewPacket(FIN_PACKET, FIN_FLAG, 9999, 10000, []byte{}),
			shouldFail: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize the packet
			serialized := tc.packet.Serialize()
			
			// Verify serialized length matches packet length
			if len(serialized) != int(tc.packet.Length) {
				t.Errorf("Serialized length mismatch: expected %d, got %d", 
					tc.packet.Length, len(serialized))
			}

			// Deserialize the packet
			deserialized, err := DeserializePacket(serialized)
			
			if tc.shouldFail {
				if err == nil {
					t.Error("Expected deserialization to fail, but it succeeded")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Deserialization failed: %v", err)
			}

			// Compare all fields
			if deserialized.Version != tc.packet.Version {
				t.Errorf("Version mismatch: expected %d, got %d", 
					tc.packet.Version, deserialized.Version)
			}
			if deserialized.Type != tc.packet.Type {
				t.Errorf("Type mismatch: expected %d, got %d", 
					tc.packet.Type, deserialized.Type)
			}
			if deserialized.Flags != tc.packet.Flags {
				t.Errorf("Flags mismatch: expected %d, got %d", 
					tc.packet.Flags, deserialized.Flags)
			}
			if deserialized.Length != tc.packet.Length {
				t.Errorf("Length mismatch: expected %d, got %d", 
					tc.packet.Length, deserialized.Length)
			}
			if deserialized.SeqNum != tc.packet.SeqNum {
				t.Errorf("SeqNum mismatch: expected %d, got %d", 
					tc.packet.SeqNum, deserialized.SeqNum)
			}
			if deserialized.AckNum != tc.packet.AckNum {
				t.Errorf("AckNum mismatch: expected %d, got %d", 
					tc.packet.AckNum, deserialized.AckNum)
			}
			if deserialized.Checksum != tc.packet.Checksum {
				t.Errorf("Checksum mismatch: expected 0x%08X, got 0x%08X", 
					tc.packet.Checksum, deserialized.Checksum)
			}
			if !bytes.Equal(deserialized.Payload, tc.packet.Payload) {
				t.Errorf("Payload mismatch: expected %v, got %v", 
					tc.packet.Payload, deserialized.Payload)
			}
		})
	}
}

// Test packet type checking methods
func TestPacketTypeChecking(t *testing.T) {
	testCases := []struct {
		packetType uint8
		flags      uint8
		tests      map[string]func(*Packet) bool
		expected   map[string]bool
	}{
		{
			packetType: DATA_PACKET,
			flags:      0,
			tests: map[string]func(*Packet) bool{
				"IsDataPacket": (*Packet).IsDataPacket,
				"IsAckPacket":  (*Packet).IsAckPacket,
				"IsSynPacket":  (*Packet).IsSynPacket,
				"IsFinPacket":  (*Packet).IsFinPacket,
				"IsRstPacket":  (*Packet).IsRstPacket,
			},
			expected: map[string]bool{
				"IsDataPacket": true,
				"IsAckPacket":  false,
				"IsSynPacket":  false,
				"IsFinPacket":  false,
				"IsRstPacket":  false,
			},
		},
		{
			packetType: ACK_PACKET,
			flags:      ACK_FLAG,
			tests: map[string]func(*Packet) bool{
				"IsAckPacket": (*Packet).IsAckPacket,
				"HasAck":      (*Packet).HasAck,
				"HasSyn":      (*Packet).HasSyn,
				"HasFin":      (*Packet).HasFin,
				"HasRst":      (*Packet).HasRst,
			},
			expected: map[string]bool{
				"IsAckPacket": true,
				"HasAck":      true,
				"HasSyn":      false,
				"HasFin":      false,
				"HasRst":      false,
			},
		},
		{
			packetType: SYN_PACKET,
			flags:      SYN_FLAG | ACK_FLAG,
			tests: map[string]func(*Packet) bool{
				"IsSynPacket": (*Packet).IsSynPacket,
				"HasSyn":      (*Packet).HasSyn,
				"HasAck":      (*Packet).HasAck,
			},
			expected: map[string]bool{
				"IsSynPacket": true,
				"HasSyn":      true,
				"HasAck":      true,
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("PacketType_%d", i), func(t *testing.T) {
			packet := NewPacket(tc.packetType, tc.flags, 1000, 2000, []byte("test"))
			
			for testName, testFunc := range tc.tests {
				expected := tc.expected[testName]
				actual := testFunc(packet)
				if actual != expected {
					t.Errorf("%s: expected %v, got %v", testName, expected, actual)
				}
			}
		})
	}
}

// Test checksum calculation
func TestChecksumCalculation(t *testing.T) {
	testCases := []struct {
		name    string
		packet  *Packet
		corrupt func([]byte) []byte // Function to corrupt the packet
		valid   bool
	}{
		{
			name:   "Valid packet with correct checksum",
			packet: NewPacket(DATA_PACKET, 0, 1000, 2000, []byte("test data")),
			corrupt: func(data []byte) []byte { return data }, // No corruption
			valid:  true,
		},
		{
			name:   "Corrupted header",
			packet: NewPacket(DATA_PACKET, 0, 1000, 2000, []byte("test data")),
			corrupt: func(data []byte) []byte {
				corrupted := make([]byte, len(data))
				copy(corrupted, data)
				corrupted[1] = ^corrupted[1] // Flip flags byte
				return corrupted
			},
			valid: false,
		},
		{
			name:   "Corrupted payload",
			packet: NewPacket(DATA_PACKET, 0, 1000, 2000, []byte("test data")),
			corrupt: func(data []byte) []byte {
				corrupted := make([]byte, len(data))
				copy(corrupted, data)
				if len(corrupted) > PACKET_HEADER_SIZE {
					corrupted[PACKET_HEADER_SIZE] = ^corrupted[PACKET_HEADER_SIZE]
				}
				return corrupted
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize original packet
			original := tc.packet.Serialize()
			
			// Apply corruption
			corrupted := tc.corrupt(original)
			
			// Try to deserialize
			_, err := DeserializePacket(corrupted)
			
			if tc.valid && err != nil {
				t.Errorf("Expected valid packet, but got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Error("Expected invalid packet due to checksum mismatch, but deserialization succeeded")
			}
		})
	}
}

// Test packet string representation
func TestPacketString(t *testing.T) {
	testCases := []struct {
		name     string
		packet   *Packet
		contains []string // Substrings that should be in the string representation
	}{
		{
			name:     "Data packet",
			packet:   NewPacket(DATA_PACKET, 0, 1000, 2000, []byte("hello")),
			contains: []string{"DATA", "seq=1000", "ack=2000", "payload=5"},
		},
		{
			name:     "ACK packet with flag",
			packet:   NewPacket(ACK_PACKET, ACK_FLAG, 0, 1001, nil),
			contains: []string{"ACK", "[ACK]", "seq=0", "ack=1001", "payload=0"},
		},
		{
			name:     "SYN+ACK packet",
			packet:   NewPacket(SYN_PACKET, SYN_FLAG|ACK_FLAG, 100, 200, []byte("handshake")),
			contains: []string{"SYN", "[SYN,ACK]", "seq=100", "ack=200"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			str := tc.packet.String()
			
			for _, substring := range tc.contains {
				if !containsString(str, substring) {
					t.Errorf("String representation '%s' should contain '%s'", str, substring)
				}
			}
		})
	}
}

// Test error conditions in deserialization
func TestDeserializationErrors(t *testing.T) {
	testCases := []struct {
		name        string
		data        []byte
		expectError string
	}{
		{
			name:        "Too short packet",
			data:        make([]byte, PACKET_HEADER_SIZE-1),
			expectError: "packet too short",
		},
		{
			name: "Invalid protocol version",
			data: func() []byte {
				packet := NewPacket(DATA_PACKET, 0, 1000, 2000, []byte("test"))
				data := packet.Serialize()
				data[0] = (0x02 << 4) | (DATA_PACKET & 0x0F) // Wrong version
				return data
			}(),
			expectError: "unsupported protocol version",
		},
		{
			name: "Length mismatch",
			data: func() []byte {
				packet := NewPacket(DATA_PACKET, 0, 1000, 2000, []byte("test"))
				data := packet.Serialize()
				// Create data with wrong length
				shortData := make([]byte, len(data)-2)
				copy(shortData, data)
				return shortData
			}(),
			expectError: "packet length mismatch",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DeserializePacket(tc.data)
			if err == nil {
				t.Error("Expected an error, but deserialization succeeded")
				return
			}
			
			if !containsString(err.Error(), tc.expectError) {
				t.Errorf("Expected error containing '%s', got '%s'", tc.expectError, err.Error())
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}