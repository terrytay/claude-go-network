package main

import (
	"fmt"
	"unsafe"
)

// Packet types
const (
	DATA_PACKET = 0x01
	ACK_PACKET  = 0x02
	SYN_PACKET  = 0x03
	FIN_PACKET  = 0x04
	RST_PACKET  = 0x05
)

// Packet flags
const (
	ACK_FLAG = 0x01
	SYN_FLAG = 0x02
	FIN_FLAG = 0x04
	RST_FLAG = 0x08
)

// Protocol constants
const (
	PROTOCOL_VERSION = 0x01
	PACKET_HEADER_SIZE = 16  // 16 bytes header
	MAX_PAYLOAD_SIZE = 1400  // MTU - IP header - UDP header - our header
	MAX_PACKET_SIZE = PACKET_HEADER_SIZE + MAX_PAYLOAD_SIZE
)

// Packet represents our custom protocol packet
type Packet struct {
	Version    uint8   // Protocol version (4 bits) + Type (4 bits)
	Type       uint8   // Packet type
	Flags      uint8   // Control flags
	Length     uint16  // Total packet length
	SeqNum     uint32  // Sequence number
	AckNum     uint32  // Acknowledgment number
	Checksum   uint32  // Packet checksum
	Payload    []byte  // Packet payload
}

// NewPacket creates a new packet with the specified parameters
func NewPacket(packetType uint8, flags uint8, seqNum uint32, ackNum uint32, payload []byte) *Packet {
	if len(payload) > MAX_PAYLOAD_SIZE {
		payload = payload[:MAX_PAYLOAD_SIZE]
	}

	return &Packet{
		Version:  PROTOCOL_VERSION,
		Type:     packetType,
		Flags:    flags,
		Length:   uint16(PACKET_HEADER_SIZE + len(payload)),
		SeqNum:   seqNum,
		AckNum:   ackNum,
		Checksum: 0, // Will be calculated during serialization
		Payload:  payload,
	}
}

// Serialize converts the packet to byte array for transmission
func (p *Packet) Serialize() []byte {
	buffer := make([]byte, p.Length)
	
	// Pack header fields in network byte order
	buffer[0] = (p.Version << 4) | (p.Type & 0x0F)
	buffer[1] = p.Flags
	*(*uint16)(unsafe.Pointer(&buffer[2])) = htons(p.Length)
	*(*uint32)(unsafe.Pointer(&buffer[4])) = htonl(p.SeqNum)
	*(*uint32)(unsafe.Pointer(&buffer[8])) = htonl(p.AckNum)
	
	// Copy payload
	if len(p.Payload) > 0 {
		copy(buffer[PACKET_HEADER_SIZE:], p.Payload)
	}
	
	// Calculate and set checksum (exclude checksum field itself)
	p.Checksum = calculateChecksum(buffer[:12], buffer[PACKET_HEADER_SIZE:])
	*(*uint32)(unsafe.Pointer(&buffer[12])) = htonl(p.Checksum)
	
	return buffer
}

// Deserialize converts byte array back to packet structure
func DeserializePacket(data []byte) (*Packet, error) {
	if len(data) < PACKET_HEADER_SIZE {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}
	
	p := &Packet{}
	
	// Unpack header fields from network byte order
	versionType := data[0]
	p.Version = (versionType >> 4) & 0x0F
	p.Type = versionType & 0x0F
	p.Flags = data[1]
	p.Length = ntohs(*(*uint16)(unsafe.Pointer(&data[2])))
	p.SeqNum = ntohl(*(*uint32)(unsafe.Pointer(&data[4])))
	p.AckNum = ntohl(*(*uint32)(unsafe.Pointer(&data[8])))
	p.Checksum = ntohl(*(*uint32)(unsafe.Pointer(&data[12])))
	
	// Validate packet length
	if int(p.Length) != len(data) {
		return nil, fmt.Errorf("packet length mismatch: expected %d, got %d", p.Length, len(data))
	}
	
	// Validate protocol version
	if p.Version != PROTOCOL_VERSION {
		return nil, fmt.Errorf("unsupported protocol version: %d", p.Version)
	}
	
	// Extract payload
	if p.Length > PACKET_HEADER_SIZE {
		payloadLen := p.Length - PACKET_HEADER_SIZE
		p.Payload = make([]byte, payloadLen)
		copy(p.Payload, data[PACKET_HEADER_SIZE:])
	}
	
	// Verify checksum
	expectedChecksum := calculateChecksum(data[:12], data[PACKET_HEADER_SIZE:])
	if p.Checksum != expectedChecksum {
		return nil, fmt.Errorf("checksum mismatch: expected 0x%08X, got 0x%08X", 
			expectedChecksum, p.Checksum)
	}
	
	return p, nil
}

// calculateChecksum computes a simple checksum for the packet
// This is a basic implementation - in production, use CRC32 or similar
func calculateChecksum(header []byte, payload []byte) uint32 {
	var sum uint32
	
	// Checksum header (excluding checksum field)
	for i := 0; i < len(header); i += 4 {
		if i+4 <= len(header) {
			word := ntohl(*(*uint32)(unsafe.Pointer(&header[i])))
			sum += word
		} else {
			// Handle remaining bytes
			word := uint32(0)
			for j := i; j < len(header); j++ {
				word |= uint32(header[j]) << (8 * (3 - (j - i)))
			}
			sum += word
		}
	}
	
	// Checksum payload
	for i := 0; i < len(payload); i += 4 {
		if i+4 <= len(payload) {
			word := ntohl(*(*uint32)(unsafe.Pointer(&payload[i])))
			sum += word
		} else {
			// Handle remaining bytes
			word := uint32(0)
			for j := i; j < len(payload); j++ {
				word |= uint32(payload[j]) << (8 * (3 - (j - i)))
			}
			sum += word
		}
	}
	
	// Fold carry bits
	for (sum >> 16) > 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	
	return ^sum & 0xFFFFFFFF
}

// IsDataPacket returns true if this is a data packet
func (p *Packet) IsDataPacket() bool {
	return p.Type == DATA_PACKET
}

// IsAckPacket returns true if this is an acknowledgment packet
func (p *Packet) IsAckPacket() bool {
	return p.Type == ACK_PACKET
}

// IsSynPacket returns true if this is a synchronize packet
func (p *Packet) IsSynPacket() bool {
	return p.Type == SYN_PACKET
}

// IsFinPacket returns true if this is a finish packet
func (p *Packet) IsFinPacket() bool {
	return p.Type == FIN_PACKET
}

// IsRstPacket returns true if this is a reset packet
func (p *Packet) IsRstPacket() bool {
	return p.Type == RST_PACKET
}

// HasAck returns true if ACK flag is set
func (p *Packet) HasAck() bool {
	return (p.Flags & ACK_FLAG) != 0
}

// HasSyn returns true if SYN flag is set
func (p *Packet) HasSyn() bool {
	return (p.Flags & SYN_FLAG) != 0
}

// HasFin returns true if FIN flag is set
func (p *Packet) HasFin() bool {
	return (p.Flags & FIN_FLAG) != 0
}

// HasRst returns true if RST flag is set
func (p *Packet) HasRst() bool {
	return (p.Flags & RST_FLAG) != 0
}

// String returns a human-readable representation of the packet
func (p *Packet) String() string {
	typeStr := ""
	switch p.Type {
	case DATA_PACKET:
		typeStr = "DATA"
	case ACK_PACKET:
		typeStr = "ACK"
	case SYN_PACKET:
		typeStr = "SYN"
	case FIN_PACKET:
		typeStr = "FIN"
	case RST_PACKET:
		typeStr = "RST"
	default:
		typeStr = fmt.Sprintf("UNKNOWN(%d)", p.Type)
	}
	
	flags := []string{}
	if p.HasSyn() {
		flags = append(flags, "SYN")
	}
	if p.HasAck() {
		flags = append(flags, "ACK")
	}
	if p.HasFin() {
		flags = append(flags, "FIN")
	}
	if p.HasRst() {
		flags = append(flags, "RST")
	}
	
	flagStr := ""
	if len(flags) > 0 {
		flagStr = fmt.Sprintf(" [%s]", joinStrings(flags, ","))
	}
	
	return fmt.Sprintf("%s%s seq=%d ack=%d len=%d payload=%d", 
		typeStr, flagStr, p.SeqNum, p.AckNum, p.Length, len(p.Payload))
}

// Helper function to join strings (since we can't use strings package)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}