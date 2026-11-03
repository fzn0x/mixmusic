package cast

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// message is a CastMessage, the protocol buffer every message between a
// sender and a cast device travels in:
//
//	message CastMessage {
//	  required ProtocolVersion protocol_version = 1; // CASTV2_1_0 = 0
//	  required string source_id = 2;
//	  required string destination_id = 3;
//	  required string namespace = 4;
//	  required PayloadType payload_type = 5;         // STRING = 0, BINARY = 1
//	  optional string payload_utf8 = 6;
//	  optional bytes payload_binary = 7;
//	}
//
// It is written by hand: seven fields don't need a protobuf library. Its
// fields are proto2's required ones, which are always written, zeros too.
type message struct {
	Source, Destination, Namespace string
	// Binary is set for a payload of bytes, else Payload is a string: JSON,
	// in every namespace MixMusic speaks.
	Binary  bool
	Payload []byte
}

// The wire types of protocol buffers that a CastMessage has.
const (
	wireVarint = 0
	wire64     = 1
	wireBytes  = 2
	wire32     = 5
)

// maxMessage bounds the size of a message read: the protocol's are below
// 64 KiB.
const maxMessage = 1 << 20

// marshal returns the protocol buffer of m.
func (m message) marshal() []byte {
	b := make([]byte, 0, 32+len(m.Source)+len(m.Destination)+len(m.Namespace)+len(m.Payload))
	b = appendVarintField(b, 1, 0)
	b = appendBytesField(b, 2, []byte(m.Source))
	b = appendBytesField(b, 3, []byte(m.Destination))
	b = appendBytesField(b, 4, []byte(m.Namespace))
	if m.Binary {
		b = appendVarintField(b, 5, 1)
		b = appendBytesField(b, 7, m.Payload)
	} else {
		b = appendVarintField(b, 5, 0)
		b = appendBytesField(b, 6, m.Payload)
	}
	return b
}

func appendVarintField(b []byte, field int, v uint64) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3|wireVarint)
	return binary.AppendUvarint(b, v)
}

func appendBytesField(b []byte, field int, v []byte) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3|wireBytes)
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}

var errTruncated = errors.New("cast: truncated message")

// unmarshal reads the protocol buffer of a CastMessage, skipping fields it
// doesn't know, as a newer device may send.
func unmarshal(b []byte) (message, error) {
	var m message
	for len(b) > 0 {
		key, n := binary.Uvarint(b)
		if n <= 0 {
			return m, errTruncated
		}
		b = b[n:]
		field, wire := key>>3, key&7
		switch wire {
		case wireVarint:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return m, errTruncated
			}
			b = b[n:]
			if field == 5 {
				m.Binary = v == 1
			}
		case wireBytes:
			l, n := binary.Uvarint(b)
			if n <= 0 || l > uint64(len(b)-n) {
				return m, errTruncated
			}
			v := b[n : n+int(l)]
			b = b[n+int(l):]
			switch field {
			case 2:
				m.Source = string(v)
			case 3:
				m.Destination = string(v)
			case 4:
				m.Namespace = string(v)
			case 6, 7:
				m.Payload = append([]byte(nil), v...)
			}
		case wire64:
			if len(b) < 8 {
				return m, errTruncated
			}
			b = b[8:]
		case wire32:
			if len(b) < 4 {
				return m, errTruncated
			}
			b = b[4:]
		default:
			return m, fmt.Errorf("cast: wire type %d in a message", wire)
		}
	}
	return m, nil
}

// writeMessage writes m as the protocol frames it: its length, in four
// bytes, big-endian, then its protocol buffer.
func writeMessage(w io.Writer, m message) error {
	body := m.marshal()
	frame := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(frame, uint32(len(body)))
	_, err := w.Write(append(frame, body...))
	return err
}

// readMessage reads a message framed as writeMessage writes it.
func readMessage(r io.Reader) (message, error) {
	var size [4]byte
	if _, err := io.ReadFull(r, size[:]); err != nil {
		return message{}, err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n > maxMessage {
		return message{}, fmt.Errorf("cast: a message of %d bytes", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return message{}, err
	}
	return unmarshal(body)
}
