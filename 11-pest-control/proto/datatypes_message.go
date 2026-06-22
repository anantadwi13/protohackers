package proto

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrMessageInvalidChecksum = errors.New("invalid message checksum")
	ErrMessageInvalidLength   = errors.New("invalid message length")
)

type Message interface {
	DataType
	Id() Byte
}

type SiteId U32

type PolicyId U32

type Species String

type PolicyAction Byte

const (
	PolicyActionCull     = PolicyAction(0x90)
	PolicyActionConserve = PolicyAction(0xa0)
)

type TargetPopulationsPopulation struct {
	Species Species
	Min     U32
	Max     U32
}

type SiteVisitPopulation struct {
	Species Species
	Count   U32
}

type MessageHello struct {
	Protocol String
	Version  U32
}

func (m *MessageHello) BytesLength() uint32 {
	var length uint32 = 1 + 4 + 1
	length += m.Protocol.BytesLength()
	length += m.Version.BytesLength()
	return length
}

func (m *MessageHello) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshaler(m, w, &m.Protocol, &m.Version)

	var (
		buf = make([]byte, 4) // todo use pool
	)

	_, err = w.Write([]byte{byte(m.Id())})
	preChecksum += byte(m.Id())
	if err != nil {
		return
	}

	binary.BigEndian.PutUint32(buf, uint32(m.BytesLength()))
	_, err = w.Write(buf)
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}

	pcs, err := m.Protocol.Marshal(w)
	preChecksum += pcs
	if err != nil {
		return
	}

	pcs, err = m.Version.Marshal(w)
	preChecksum += pcs
	if err != nil {
		return
	}

	checksum := byte(0) - preChecksum

	_, err = w.Write([]byte{checksum})
	preChecksum += checksum
	if err != nil {
		return
	}

	return
}

func (m *MessageHello) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshaler(m, r, &m.Protocol, &m.Version)

	var (
		buf       = make([]byte, 1) // todo use pool
		msgLength uint32
	)

	newN, err := io.ReadFull(r, buf)
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}
	if buf[0] != byte(m.Id()) {
		err = ErrInvalidType
		return
	}

	buf = make([]byte, 4) // todo use pool
	newN, err = io.ReadFull(r, buf)
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}
	msgLength = binary.BigEndian.Uint32(buf)

	var pcs byte
	newN, pcs, err = m.Protocol.Unmarshal(r)
	n += newN
	preChecksum += pcs
	if err != nil {
		return
	}

	newN, pcs, err = m.Version.Unmarshal(r)
	n += newN
	preChecksum += pcs
	if err != nil {
		return
	}

	buf = make([]byte, 1) // todo use pool
	newN, err = io.ReadFull(r, buf)
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}
	if preChecksum != byte(0) {
		err = ErrMessageInvalidChecksum
		return
	}

	if msgLength != uint32(n) {
		err = ErrMessageInvalidLength
		return
	}

	return
}

func (m *MessageHello) Reset() {
	m.Protocol.Reset()
	m.Version.Reset()
}

func (m *MessageHello) Id() Byte {
	return Byte(0x50)
}

type MessageError struct {
	Message String
}

type MessageOk struct {
}

type MessageDialAuthority struct {
	Site SiteId
}

type MessageTargetPopulations struct {
	Site        SiteId
	Populations []TargetPopulationsPopulation
}

type MessageCreatePolicy struct {
	Species Species
	Action  PolicyAction
}

type MessageDeletePolicy struct {
	Policy PolicyId
}

type MessagePolicyResult struct {
	Policy PolicyId
}

type MessageSiteVisit struct {
	Site        SiteId
	Populations []SiteVisitPopulation
}

func messageMarshaler(m Message, w io.Writer, fields ...DataType) (preChecksum byte, err error) {
	var (
		buf = make([]byte, 4) // todo use pool
	)

	_, err = w.Write([]byte{byte(m.Id())})
	preChecksum += byte(m.Id())
	if err != nil {
		return
	}

	binary.BigEndian.PutUint32(buf, uint32(m.BytesLength()))
	_, err = w.Write(buf)
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}

	var pcs byte
	for _, field := range fields {
		pcs, err = field.Marshal(w)
		preChecksum += pcs
		if err != nil {
			return
		}
	}

	checksum := byte(0) - preChecksum

	_, err = w.Write([]byte{checksum})
	preChecksum += checksum
	if err != nil {
		return
	}

	return
}

func messageUnmarshaler(m Message, r io.Reader, fields ...DataType) (n int, preChecksum byte, err error) {
	var (
		buf       = make([]byte, 1) // todo use pool
		msgLength uint32
	)

	newN, err := io.ReadFull(r, buf)
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}
	if buf[0] != byte(m.Id()) {
		err = ErrInvalidType
		return
	}

	buf = make([]byte, 4) // todo use pool
	newN, err = io.ReadFull(r, buf)
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}
	msgLength = binary.BigEndian.Uint32(buf)

	var pcs byte
	for _, field := range fields {
		newN, pcs, err = field.Unmarshal(r)
		n += newN
		preChecksum += pcs
		if err != nil {
			return
		}
	}

	buf = make([]byte, 1) // todo use pool
	newN, err = io.ReadFull(r, buf)
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return
	}
	if preChecksum != byte(0) {
		err = ErrMessageInvalidChecksum
		return
	}

	if msgLength != uint32(n) {
		err = ErrMessageInvalidLength
		return
	}

	return
}
