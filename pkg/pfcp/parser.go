// Package pfcp – PFCP binary message parser (req 2.1, 2.2).
package pfcp

import (
	"encoding/binary"
	"fmt"
	"net"
)

// MsgType is a PFCP message type code (3GPP TS 29.244 Table 7.2.1-1).
type MsgType uint8

const (
	MsgTypeHeartbeatRequest             MsgType = 1
	MsgTypeHeartbeatResponse            MsgType = 2
	MsgTypeAssociationSetupRequest      MsgType = 5
	MsgTypeAssociationSetupResponse     MsgType = 6
	MsgTypeSessionEstablishmentRequest  MsgType = 50
	MsgTypeSessionEstablishmentResponse MsgType = 51
	MsgTypeSessionModificationRequest   MsgType = 52
	MsgTypeSessionModificationResponse  MsgType = 53
	MsgTypeSessionDeletionRequest       MsgType = 54
	MsgTypeSessionDeletionResponse      MsgType = 55
)

// ParseError is returned when a PFCP message cannot be parsed.
type ParseError struct{ Msg string }

func (e *ParseError) Error() string { return "pfcp parse: " + e.Msg }

// ParsedMessage is the result of parsing a PFCP message from wire format.
// All decoded PFCP content is nested under the "pfcp" key in Fields,
// compatible with DSL runtime field paths (e.g. pfcp.seid, pfcp.create_pdr[*].pdr_id).
type ParsedMessage struct {
	Type   MsgType
	SEID   uint64
	SeqNo  uint32
	Fields map[string]interface{} // top level: {"pfcp": {...}}
}

// ToEstablishmentRequest wraps the message as a PFCPEstablishmentRequest.
func (m *ParsedMessage) ToEstablishmentRequest() *PFCPEstablishmentRequest {
	return &PFCPEstablishmentRequest{SEID: m.SEID, Fields: m.Fields}
}

// ToModificationRequest wraps the message as a PFCPModificationRequest.
func (m *ParsedMessage) ToModificationRequest() *PFCPModificationRequest {
	return &PFCPModificationRequest{SEID: m.SEID, Fields: m.Fields}
}

// ToDeletionRequest wraps the message as a PFCPDeletionRequest.
func (m *ParsedMessage) ToDeletionRequest() *PFCPDeletionRequest {
	return &PFCPDeletionRequest{SEID: m.SEID}
}

// ParseMessage parses a raw UDP payload as a PFCP message (3GPP TS 29.244 §7.2).
// Returns an error if data is malformed or truncated; the error is non-nil and the
// caller may log it and continue processing subsequent packets (req 9.4).
func ParseMessage(data []byte) (*ParsedMessage, error) {
	if len(data) < 4 {
		return nil, &ParseError{Msg: fmt.Sprintf("too short: %d bytes", len(data))}
	}

	version := (data[0] >> 5) & 0x7
	if version != 1 {
		return nil, &ParseError{Msg: fmt.Sprintf("unsupported version %d", version)}
	}
	sFlag := (data[0] & 0x01) != 0
	msgType := MsgType(data[1])
	msgLen := int(binary.BigEndian.Uint16(data[2:4]))

	if len(data) < msgLen+4 {
		return nil, &ParseError{Msg: fmt.Sprintf("truncated: have %d, need %d", len(data), msgLen+4)}
	}

	msg := &ParsedMessage{Type: msgType, Fields: make(map[string]interface{})}

	var ieStart int
	if sFlag {
		if len(data) < 16 {
			return nil, &ParseError{Msg: "SEID present but header too short"}
		}
		msg.SEID = binary.BigEndian.Uint64(data[4:12])
		msg.SeqNo = uint32(data[12])<<16 | uint32(data[13])<<8 | uint32(data[14])
		ieStart = 16
	} else {
		msg.SeqNo = uint32(data[4])<<16 | uint32(data[5])<<8 | uint32(data[6])
		ieStart = 8
	}

	ieEnd := 4 + msgLen
	pfcpFields := map[string]interface{}{
		"msg_type": uint8(msgType),
		"seid":     msg.SEID,
		"seqno":    msg.SeqNo,
	}

	ies, err := parseIEList(data[ieStart:ieEnd])
	if err != nil {
		return nil, fmt.Errorf("pfcp parse: IEs: %w", err)
	}
	mergeIEs(pfcpFields, ies)
	msg.Fields["pfcp"] = pfcpFields
	return msg, nil
}

// --- IE parsing -------------------------------------------------------------

type rawIE struct {
	typ  uint16
	data []byte
}

func parseIEList(data []byte) ([]rawIE, error) {
	var ies []rawIE
	for i := 0; i+4 <= len(data); {
		t := binary.BigEndian.Uint16(data[i : i+2])
		l := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if i+4+l > len(data) {
			return nil, &ParseError{Msg: fmt.Sprintf("IE type %d: length %d exceeds data", t, l)}
		}
		ies = append(ies, rawIE{typ: t, data: data[i+4 : i+4+l]})
		i += 4 + l
	}
	return ies, nil
}

// mergeIEs decodes each raw IE and adds the result to fields.
// Grouped IEs are decoded recursively; arrays (Create PDR, etc.) are appended.
func mergeIEs(fields map[string]interface{}, ies []rawIE) {
	for _, ie := range ies {
		switch ie.typ {
		// Grouped IEs – collected as []interface{}
		case 1:
			appendGrouped(fields, "create_pdr", ie.data)
		case 3:
			appendGrouped(fields, "create_far", ie.data)
		case 7:
			appendGrouped(fields, "create_qer", ie.data)
		case 8:
			appendGrouped(fields, "created_pdr", ie.data)
		case 10:
			appendGrouped(fields, "update_far", ie.data)
		case 13:
			appendGrouped(fields, "update_qer", ie.data)
		case 15:
			appendGrouped(fields, "remove_pdr", ie.data)
		case 16:
			appendGrouped(fields, "remove_far", ie.data)

		// Single grouped container IEs
		case 2: // PDI
			fields["pdi"] = decodeGrouped(ie.data)
		case 4: // Forwarding Parameters
			fields["forwarding_parameters"] = decodeGrouped(ie.data)
		case 11: // Update Forwarding Parameters
			fields["update_forwarding_parameters"] = decodeGrouped(ie.data)

		// Scalar IEs
		case 19: // Cause
			if len(ie.data) >= 1 {
				fields["cause"] = uint8(ie.data[0])
			}
		case 20: // Source Interface
			if len(ie.data) >= 1 {
				fields["source_interface"] = uint8(ie.data[0] & 0x0F)
			}
		case 21: // F-TEID
			if m := parseFTEID(ie.data); m != nil {
				fields["f_teid"] = m
			}
		case 22: // Network Instance
			fields["network_instance"] = string(ie.data)
		case 29: // Precedence
			if len(ie.data) >= 4 {
				fields["precedence"] = binary.BigEndian.Uint32(ie.data[:4])
			}
		case 42: // Destination Interface
			if len(ie.data) >= 1 {
				fields["destination_interface"] = uint8(ie.data[0] & 0x0F)
			}
		case 44: // Apply Action
			if len(ie.data) >= 2 {
				fields["apply_action"] = binary.BigEndian.Uint16(ie.data[:2])
			}
		case 56: // PDR ID
			if len(ie.data) >= 2 {
				fields["pdr_id"] = binary.BigEndian.Uint16(ie.data[:2])
			}
		case 57: // F-SEID
			if m := parseFSEID(ie.data); m != nil {
				fields["f_seid"] = m
			}
		case 60: // Node ID
			if m := parseNodeID(ie.data); m != nil {
				fields["node_id"] = m
			}
		case 84: // Outer Header Creation
			if m := parseOuterHeaderCreation(ie.data); m != nil {
				fields["outer_header_creation"] = m
			}
		case 93: // UE IP Address
			if m := parseUEIPAddress(ie.data); m != nil {
				fields["ue_ip_address"] = m
			}
		case 108: // FAR ID
			if len(ie.data) >= 4 {
				fields["far_id"] = binary.BigEndian.Uint32(ie.data[:4])
			}
		case 109: // QER ID
			if len(ie.data) >= 4 {
				fields["qer_id"] = binary.BigEndian.Uint32(ie.data[:4])
			}
		}
	}
}

func decodeGrouped(data []byte) map[string]interface{} {
	sub := make(map[string]interface{})
	if ies, err := parseIEList(data); err == nil {
		mergeIEs(sub, ies)
	}
	return sub
}

func appendGrouped(fields map[string]interface{}, key string, data []byte) {
	sub := decodeGrouped(data)
	existing, _ := fields[key].([]interface{})
	fields[key] = append(existing, sub)
}

// --- Scalar IE decoders -----------------------------------------------------

func parseFSEID(data []byte) map[string]interface{} {
	if len(data) < 9 {
		return nil
	}
	flags := data[0]
	seid := binary.BigEndian.Uint64(data[1:9])
	m := map[string]interface{}{"seid": seid}
	off := 9
	if flags&0x02 != 0 && off+4 <= len(data) { // IPv4
		m["ipv4"] = net.IP(data[off : off+4]).String()
		off += 4
	}
	if flags&0x01 != 0 && off+16 <= len(data) { // IPv6
		m["ipv6"] = net.IP(data[off : off+16]).String()
	}
	return m
}

func parseFTEID(data []byte) map[string]interface{} {
	if len(data) < 1 {
		return nil
	}
	flags := data[0]
	m := map[string]interface{}{
		"ch":    (flags & 0x04) != 0,
		"ch_id": (flags & 0x02) != 0,
		"v4":    (flags & 0x01) != 0,
		"v6":    (flags & 0x08) != 0,
	}
	if flags&0x04 != 0 { // CHOOSE bit set
		if len(data) >= 2 {
			m["choose_id"] = data[1]
		}
		return m
	}
	off := 1
	if off+4 <= len(data) {
		m["teid"] = binary.BigEndian.Uint32(data[off : off+4])
		off += 4
	}
	if flags&0x01 != 0 && off+4 <= len(data) {
		m["ipv4"] = net.IP(data[off : off+4]).String()
		off += 4
	}
	if flags&0x08 != 0 && off+16 <= len(data) {
		m["ipv6"] = net.IP(data[off : off+16]).String()
	}
	return m
}

func parseNodeID(data []byte) map[string]interface{} {
	if len(data) < 1 {
		return nil
	}
	typ := data[0] & 0x0F
	m := map[string]interface{}{"type": typ}
	switch typ {
	case 0: // IPv4
		if len(data) >= 5 {
			m["ipv4"] = net.IP(data[1:5]).String()
		}
	case 1: // IPv6
		if len(data) >= 17 {
			m["ipv6"] = net.IP(data[1:17]).String()
		}
	case 2: // FQDN
		m["fqdn"] = string(data[1:])
	}
	return m
}

// parseOuterHeaderCreation decodes an Outer Header Creation IE.
// desc bit 8 (0x0100) = GTP-U/UDP/IPv4.
func parseOuterHeaderCreation(data []byte) map[string]interface{} {
	if len(data) < 2 {
		return nil
	}
	desc := binary.BigEndian.Uint16(data[:2])
	m := map[string]interface{}{"description": desc}
	off := 2
	if desc&0x0100 != 0 { // GTP-U/UDP/IPv4
		if off+4 <= len(data) {
			m["teid"] = binary.BigEndian.Uint32(data[off : off+4])
			off += 4
		}
		if off+4 <= len(data) {
			m["ipv4"] = net.IP(data[off : off+4]).String()
		}
	}
	return m
}

func parseUEIPAddress(data []byte) map[string]interface{} {
	if len(data) < 1 {
		return nil
	}
	flags := data[0]
	m := make(map[string]interface{})
	off := 1
	if flags&0x02 != 0 && off+4 <= len(data) { // IPv4
		m["ipv4"] = net.IP(data[off : off+4]).String()
		off += 4
	}
	if flags&0x01 != 0 && off+16 <= len(data) { // IPv6
		m["ipv6"] = net.IP(data[off : off+16]).String()
	}
	if len(m) == 0 {
		return nil
	}
	return m
}
