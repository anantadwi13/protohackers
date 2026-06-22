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

func (s *SiteId) Marshal(w io.Writer) (preChecksum byte, err error) {
	return (*U32)(s).Marshal(w)
}

func (s *SiteId) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return (*U32)(s).Unmarshal(r)
}

func (s *SiteId) Reset() {
	(*U32)(s).Reset()
}

func (s *SiteId) BytesLength() uint32 {
	return (*U32)(s).BytesLength()
}

func (s *SiteId) Value() uint32 {
	return (*U32)(s).Value()
}

type PolicyId U32

func (p *PolicyId) Marshal(w io.Writer) (preChecksum byte, err error) {
	return (*U32)(p).Marshal(w)
}

func (p *PolicyId) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return (*U32)(p).Unmarshal(r)
}

func (p *PolicyId) Reset() {
	(*U32)(p).Reset()
}

func (p *PolicyId) BytesLength() uint32 {
	return (*U32)(p).BytesLength()
}

func (p *PolicyId) Value() uint32 {
	return (*U32)(p).Value()
}

type Species String

func (s *Species) Marshal(w io.Writer) (preChecksum byte, err error) {
	return (*String)(s).Marshal(w)
}

func (s *Species) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return (*String)(s).Unmarshal(r)
}

func (s *Species) Reset() {
	(*String)(s).Reset()
}

func (s *Species) BytesLength() uint32 {
	return (*String)(s).BytesLength()
}

func (s *Species) Value() string {
	return (*String)(s).Value()
}

type PolicyAction Byte

const (
	PolicyActionCull     = PolicyAction(0x90)
	PolicyActionConserve = PolicyAction(0xa0)
)

func (p *PolicyAction) Marshal(w io.Writer) (preChecksum byte, err error) {
	return (*Byte)(p).Marshal(w)
}

func (p *PolicyAction) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return (*Byte)(p).Unmarshal(r)
}

func (p *PolicyAction) Reset() {
	(*Byte)(p).Reset()
}

func (p *PolicyAction) BytesLength() uint32 {
	return (*Byte)(p).BytesLength()
}

func (p *PolicyAction) Value() byte {
	return (*Byte)(p).Value()
}

type TargetPopulationsPopulation struct {
	Species Species
	Min     U32
	Max     U32
}

func (t *TargetPopulationsPopulation) Marshal(w io.Writer) (preChecksum byte, err error) {
	return compositeDataTypeMarshal(w, &t.Species, &t.Min, &t.Max)
}

func (t *TargetPopulationsPopulation) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return compositeDataTypeUnmarshal(r, &t.Species, &t.Min, &t.Max)
}

func (t *TargetPopulationsPopulation) Reset() {
	compositeDataTypeReset(&t.Species, &t.Min, &t.Max)
}

func (t *TargetPopulationsPopulation) BytesLength() uint32 {
	return compositeDataTypeBytesLength(&t.Species, &t.Min, &t.Max)
}

type SiteVisitPopulation struct {
	Species Species
	Count   U32
}

func (s *SiteVisitPopulation) Marshal(w io.Writer) (preChecksum byte, err error) {
	return compositeDataTypeMarshal(w, &s.Species, &s.Count)
}

func (s *SiteVisitPopulation) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return compositeDataTypeUnmarshal(r, &s.Species, &s.Count)
}

func (s *SiteVisitPopulation) Reset() {
	compositeDataTypeReset(&s.Species, &s.Count)
}

func (s *SiteVisitPopulation) BytesLength() uint32 {
	return compositeDataTypeBytesLength(&s.Species, &s.Count)
}

type MessageHello struct {
	Protocol String
	Version  U32
}

func (m *MessageHello) BytesLength() uint32 {
	return messageBytesLength(&m.Protocol, &m.Version)
}

func (m *MessageHello) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Protocol, &m.Version)
}

func (m *MessageHello) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Protocol, &m.Version)
}

func (m *MessageHello) Reset() {
	messageReset(&m.Protocol, &m.Version)
}

func (m *MessageHello) Id() Byte {
	return Byte(0x50)
}

type MessageError struct {
	Message String
}

func (m *MessageError) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Message)
}

func (m *MessageError) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Message)
}

func (m *MessageError) Reset() {
	messageReset(&m.Message)
}

func (m *MessageError) BytesLength() uint32 {
	return messageBytesLength(&m.Message)
}

func (m *MessageError) Id() Byte {
	return Byte(0x51)
}

type MessageOk struct {
}

func (m *MessageOk) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w)
}

func (m *MessageOk) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r)
}

func (m *MessageOk) Reset() {
}

func (m *MessageOk) BytesLength() uint32 {
	return messageBytesLength()
}

func (m *MessageOk) Id() Byte {
	return Byte(0x52)
}

type MessageDialAuthority struct {
	Site SiteId
}

func (m *MessageDialAuthority) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Site)
}

func (m *MessageDialAuthority) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Site)
}

func (m *MessageDialAuthority) Reset() {
	messageReset(&m.Site)
}

func (m *MessageDialAuthority) BytesLength() uint32 {
	return messageBytesLength(&m.Site)
}

func (m *MessageDialAuthority) Id() Byte {
	return Byte(0x53)
}

type MessageTargetPopulations struct {
	Site        SiteId
	Populations Array[*TargetPopulationsPopulation]
}

func (m *MessageTargetPopulations) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Site, &m.Populations)
}

func (m *MessageTargetPopulations) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Site, &m.Populations)
}

func (m *MessageTargetPopulations) Reset() {
	messageReset(&m.Site, &m.Populations)
}

func (m *MessageTargetPopulations) BytesLength() uint32 {
	return messageBytesLength(&m.Site, &m.Populations)
}

func (m *MessageTargetPopulations) Id() Byte {
	return Byte(0x54)
}

type MessageCreatePolicy struct {
	Species Species
	Action  PolicyAction
}

func (m *MessageCreatePolicy) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Species, &m.Action)
}

func (m *MessageCreatePolicy) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Species, &m.Action)
}

func (m *MessageCreatePolicy) Reset() {
	messageReset(&m.Species, &m.Action)
}

func (m *MessageCreatePolicy) BytesLength() uint32 {
	return messageBytesLength(&m.Species, &m.Action)
}

func (m *MessageCreatePolicy) Id() Byte {
	return Byte(0x55)
}

type MessageDeletePolicy struct {
	Policy PolicyId
}

func (m *MessageDeletePolicy) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Policy)
}

func (m *MessageDeletePolicy) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Policy)
}

func (m *MessageDeletePolicy) Reset() {
	messageReset(&m.Policy)
}

func (m *MessageDeletePolicy) BytesLength() uint32 {
	return messageBytesLength(&m.Policy)
}

func (m *MessageDeletePolicy) Id() Byte {
	return Byte(0x56)
}

type MessagePolicyResult struct {
	Policy PolicyId
}

func (m *MessagePolicyResult) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Policy)
}

func (m *MessagePolicyResult) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Policy)
}

func (m *MessagePolicyResult) Reset() {
	messageReset(&m.Policy)
}

func (m *MessagePolicyResult) BytesLength() uint32 {
	return messageBytesLength(&m.Policy)
}

func (m *MessagePolicyResult) Id() Byte {
	return Byte(0x57)
}

type MessageSiteVisit struct {
	Site        SiteId
	Populations Array[*SiteVisitPopulation]
}

func (m *MessageSiteVisit) Marshal(w io.Writer) (preChecksum byte, err error) {
	return messageMarshal(m, w, &m.Site, &m.Populations)
}

func (m *MessageSiteVisit) Unmarshal(r io.Reader) (n int, preChecksum byte, err error) {
	return messageUnmarshal(m, r, &m.Site, &m.Populations)
}

func (m *MessageSiteVisit) Reset() {
	m.Site.Reset()
	m.Populations.Reset()
}

func (m *MessageSiteVisit) BytesLength() uint32 {
	return messageBytesLength(&m.Site, &m.Populations)
}

func (m *MessageSiteVisit) Id() Byte {
	return Byte(0x58)
}

func compositeDataTypeMarshal(w io.Writer, fields ...DataType) (preChecksum byte, err error) {
	var pcs byte
	for _, field := range fields {
		pcs, err = field.Marshal(w)
		preChecksum += pcs
		if err != nil {
			return
		}
	}
	return
}

func compositeDataTypeUnmarshal(r io.Reader, fields ...DataType) (n int, preChecksum byte, err error) {
	var (
		pcs  byte
		newN int
	)
	for _, field := range fields {
		newN, pcs, err = field.Unmarshal(r)
		n += newN
		preChecksum += pcs
		if err != nil {
			return
		}
	}
	return
}

func compositeDataTypeReset(fields ...DataType) {
	for _, field := range fields {
		field.Reset()
	}
}

func compositeDataTypeBytesLength(fields ...DataType) uint32 {
	var length uint32 = 0
	for _, field := range fields {
		length += field.BytesLength()
	}
	return length
}

func messageBytesLength(fields ...DataType) uint32 {
	var length uint32 = 1 + 4 + 1
	for _, field := range fields {
		length += field.BytesLength()
	}
	return length
}

func messageMarshal(m Message, w io.Writer, fields ...DataType) (preChecksum byte, err error) {
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

func messageUnmarshal(m Message, r io.Reader, fields ...DataType) (n int, preChecksum byte, err error) {
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

func messageReset(fields ...DataType) {
	for _, field := range fields {
		field.Reset()
	}
}
