package lrcp

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrInvalidMessage = errors.New("invalid message")
)

type messageType uint8

const (
	messageTypeConnect messageType = iota + 1
	messageTypeData
	messageTypeAck
	messageTypeClose
)

type message struct {
	MessageType messageType
	SessionId   int32
	Length      int32
	Data        []byte
}

func (p *message) unmarshal(data []byte) error {
	var (
		lastPos = 0
		err     error
	)
	if len(data) > maxMessageSize {
		return errors.Join(errors.New("max message size exceeded"), ErrInvalidMessage)
	}
	if data[0] != '/' || data[len(data)-1] != '/' {
		return ErrInvalidMessage
	}
	messageTypeByte := make([]byte, 0, 7)
	for lastPos = lastPos + 1; ; lastPos++ {
		if data[lastPos] == '/' {
			break
		}
		messageTypeByte = append(messageTypeByte, data[lastPos])
	}

	unmarshalNumeric := func() (int32, int, error) { // parse numeric value
		lastPosNumeric := lastPos + 1
		for ; ; lastPosNumeric++ {
			if data[lastPosNumeric] == '/' {
				break
			}
		}
		val, err := strconv.ParseInt(string(data[lastPos+1:lastPosNumeric]), 10, 32)
		if err != nil {
			return 0, 0, errors.Join(err, ErrInvalidMessage)
		}
		if val < 0 {
			return 0, 0, errors.Join(errors.New("invalid val"), ErrInvalidMessage)
		}
		return int32(val), lastPosNumeric, nil
	}

	switch {
	case bytes.Equal(messageTypeByte, []byte("connect")):
		p.MessageType = messageTypeConnect
		p.SessionId, lastPos, err = unmarshalNumeric()
		if err != nil {
			return err
		}
		if lastPos+1 != len(data) {
			return errors.Join(errors.New("invalid message length"), ErrInvalidMessage)
		}
	case bytes.Equal(messageTypeByte, []byte("data")):
		p.MessageType = messageTypeData
		p.SessionId, lastPos, err = unmarshalNumeric()
		if err != nil {
			return err
		}
		p.Length, lastPos, err = unmarshalNumeric()
		if err != nil {
			return err
		}

		var escapedCharPos []int
		for i := lastPos + 1; i < len(data)-1; i++ {
			switch {
			case data[i] == '\\':
				if i+1 >= len(data)-1 {
					return errors.Join(errors.New("invalid data"), ErrInvalidMessage)
				}
				escapedCharPos = append(escapedCharPos, i)
				i++
				continue
			case data[i] == '/':
				return errors.Join(errors.New("found char '/'"), ErrInvalidMessage)
			}
		}
		for i := range escapedCharPos {
			pos := escapedCharPos[len(escapedCharPos)-1-i]
			copy(data[pos:], data[pos+1:])
			data = data[:len(data)-1]
			// handle special char
			switch data[pos] {
			case 'n':
				data[pos] = '\n'
			case '/', '\\':
				// do nothing because it's the expected char
			}
		}
		p.Data = data[lastPos+1 : len(data)-1]
	case bytes.Equal(messageTypeByte, []byte("ack")):
		p.MessageType = messageTypeAck
		p.SessionId, lastPos, err = unmarshalNumeric()
		if err != nil {
			return err
		}
		p.Length, lastPos, err = unmarshalNumeric()
		if err != nil {
			return err
		}
		if lastPos+1 != len(data) {
			return errors.Join(errors.New("invalid message length"), ErrInvalidMessage)
		}
	case bytes.Equal(messageTypeByte, []byte("close")):
		p.MessageType = messageTypeClose
		p.SessionId, lastPos, err = unmarshalNumeric()
		if err != nil {
			return err
		}
		if lastPos+1 != len(data) {
			return errors.Join(errors.New("invalid message length"), ErrInvalidMessage)
		}
	default:
		return ErrInvalidMessage
	}
	return nil
}

func (p *message) marshal(buf []byte) (int, int, error) {
	var (
		lastPos       = 0
		lastWriteData = 0
	)

	buf = buf[:maxMessageSize]
	lastPos += copy(buf[lastPos:], "/")
	switch p.MessageType {
	case messageTypeConnect:
		lastPos += copy(buf[lastPos:], "connect/")
		lastPos += copy(buf[lastPos:], fmt.Sprintf("%d/", p.SessionId))
	case messageTypeData:
		lastPos += copy(buf[lastPos:], "data/")
		lastPos += copy(buf[lastPos:], fmt.Sprintf("%d/", p.SessionId))
		lastPos += copy(buf[lastPos:], fmt.Sprintf("%d/", p.Length))
		maxWrite := len(buf) - lastPos - 1
		lastPosData := 0
	LOOPING:
		for lastWriteData = 0; lastWriteData < len(p.Data) && lastWriteData < maxWrite; lastWriteData++ {
			switch p.Data[lastWriteData] {
			case '/', '\\', '\n':
				if lastWriteData+1 >= maxWrite {
					break LOOPING
				}
				lastPos += copy(buf[lastPos:], p.Data[lastPosData:lastWriteData])
				if p.Data[lastWriteData] == '\n' {
					lastPos += copy(buf[lastPos:], []byte{'\\', 'n'})
				} else {
					lastPos += copy(buf[lastPos:], []byte{'\\', p.Data[lastWriteData]})
				}
				lastPosData = lastWriteData + 1
			}
		}
		lastPos += copy(buf[lastPos:], p.Data[lastPosData:lastWriteData])
		lastPos += copy(buf[lastPos:], "/")
	case messageTypeAck:
		lastPos += copy(buf[lastPos:], "ack/")
		lastPos += copy(buf[lastPos:], fmt.Sprintf("%d/", p.SessionId))
		lastPos += copy(buf[lastPos:], fmt.Sprintf("%d/", p.Length))
	case messageTypeClose:
		lastPos += copy(buf[lastPos:], "close/")
		lastPos += copy(buf[lastPos:], fmt.Sprintf("%d/", p.SessionId))
	default:
		return 0, 0, ErrInvalidMessage
	}
	return lastPos, lastWriteData, nil
}
