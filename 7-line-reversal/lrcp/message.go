package lrcp

import (
	"errors"
	"fmt"
	"github.com/anantadwi13/protohackers/7-line-reversal/util"
	"io"
	"strconv"
)

const (
	MTU             = 1000 - 1
	actionMaxLength = 9
)

var (
	ErrMessageTooLarge = errors.New("error message is too large")
	ErrMessageInvalid  = errors.New("error message is invalid")
)

type Message interface {
	SessionNumber() int32
	Marshal(writer io.Writer) error
	Unmarshaler(reader io.Reader) error
	MarshalSize() int
}

func UnmarshalMessage(reader io.Reader) (msg Message, err error) {
	var (
		action = make([]byte, 0, actionMaxLength) // todo: use pool
		buf    = make([]byte, 1)
		n      int
	)

	msg = &MessageUnknown{}
	defer func() {
		if err != nil {
			msg = &MessageUnknown{}
		}
	}()

	// get action
	foundSlash := 2
	for {
		if foundSlash == 0 {
			break
		}
		if len(action) == cap(action) {
			break
		}

		n, err = reader.Read(buf)
		if err != nil {
			return
		}
		if n != 1 {
			err = errors.New("invalid reader")
			return
		}
		if buf[0] == '/' {
			foundSlash--
		}
		action = append(action, buf[0])
	}

	switch string(action) {
	case "/connect/":
		msg = &MessageConnect{}
		err = msg.Unmarshaler(reader)
	case "/data/":
		msg = &MessageData{}
		err = msg.Unmarshaler(reader)
	case "/ack/":
		msg = &MessageAck{}
		err = msg.Unmarshaler(reader)
	case "/close/":
		msg = &MessageClose{}
		err = msg.Unmarshaler(reader)
	default:
		err = errors.New("unknown action")
	}

	return
}

type MessageUnknown struct {
}

func (u *MessageUnknown) SessionNumber() int32 {
	return 0
}

func (u MessageUnknown) Marshal(writer io.Writer) error {
	return nil
}

func (u *MessageUnknown) Unmarshaler(reader io.Reader) error {
	return nil
}

func (u *MessageUnknown) MarshalSize() int { return 0 }

type MessageConnect struct {
	sessionNumber int32
}

func (m *MessageConnect) SessionNumber() int32 {
	return m.sessionNumber
}

func (m MessageConnect) Marshal(writer io.Writer) error {
	if m.MarshalSize() > MTU {
		return ErrMessageTooLarge
	}
	_, err := writer.Write([]byte("/connect/"))
	if err != nil {
		return err
	}
	return packToWriter(writer, m.sessionNumber)
}

func (m *MessageConnect) Unmarshaler(reader io.Reader) error {
	return unpackFromReader(reader, &m.sessionNumber)
}

func (m *MessageConnect) MarshalSize() int {
	return 9 + util.Digits(m.sessionNumber) + 1
}

type MessageData struct {
	sessionNumber int32
	Pos           int32
	Data          []byte
}

func (m *MessageData) SessionNumber() int32 {
	return m.sessionNumber
}

func (m MessageData) Marshal(writer io.Writer) error {
	if m.MarshalSize() > MTU {
		return ErrMessageTooLarge
	}
	_, err := writer.Write([]byte("/data/"))
	if err != nil {
		return err
	}
	return packToWriter(writer, m.sessionNumber, m.Pos, m.Data)
}

func (m *MessageData) Unmarshaler(reader io.Reader) error {
	return unpackFromReader(reader, &m.sessionNumber, &m.Pos, &m.Data)
}

func (m *MessageData) MarshalSize() int {
	dataLen := len(m.Data)
	for _, v := range m.Data {
		if v == '/' || v == '\\' {
			dataLen++
		}
	}

	return 9 + util.Digits(m.sessionNumber) + 1 + util.Digits(m.Pos) + 1 + dataLen + 1
}

type MessageAck struct {
	sessionNumber int32
	Length        int32
}

func (m *MessageAck) SessionNumber() int32 {
	return m.sessionNumber
}

func (m MessageAck) Marshal(writer io.Writer) error {
	if m.MarshalSize() > MTU {
		return ErrMessageTooLarge
	}
	_, err := writer.Write([]byte("/ack/"))
	if err != nil {
		return err
	}
	return packToWriter(writer, m.sessionNumber, m.Length)
}

func (m *MessageAck) Unmarshaler(reader io.Reader) error {
	return unpackFromReader(reader, &m.sessionNumber, &m.Length)
}

func (m *MessageAck) MarshalSize() int {
	return 9 + util.Digits(m.sessionNumber) + 1 + util.Digits(m.Length) + 1
}

type MessageClose struct {
	sessionNumber int32
}

func (m *MessageClose) SessionNumber() int32 {
	return m.sessionNumber
}

func (m MessageClose) Marshal(writer io.Writer) error {
	if m.MarshalSize() > MTU {
		return ErrMessageTooLarge
	}
	_, err := writer.Write([]byte("/close/"))
	if err != nil {
		return err
	}
	return packToWriter(writer, m.sessionNumber)
}

func (m *MessageClose) Unmarshaler(reader io.Reader) error {
	return unpackFromReader(reader, &m.sessionNumber)
}

func (m *MessageClose) MarshalSize() int {
	return 9 + util.Digits(m.sessionNumber) + 1
}

func packToWriter(w io.Writer, data ...any) error {
	if w == nil {
		return errors.New("nil writer")
	}

	for _, d := range data {
		switch e := d.(type) {
		case int32:
			_, err := fmt.Fprintf(w, "%d/", e)
			if err != nil {
				return err
			}
		case []byte:
			buf := make([]byte, 1)
			for _, c := range e {
				//if c == '\\' || c == '/' || c == '\n' {
				if c == '\\' || c == '/' {
					buf[0] = '\\'
					_, err := w.Write(buf)
					if err != nil {
						return err
					}
					//if c == '\n' {
					//	c = 'n'
					//}
				}
				buf[0] = c
				_, err := w.Write(buf)
				if err != nil {
					return err
				}
			}
			buf[0] = '/'
			_, err := w.Write(buf)
			if err != nil {
				return err
			}
		default:
			return errors.New("unimplemented type")
		}
	}

	return nil
}

func unpackFromReader(r io.Reader, data ...any) error {
	if r == nil {
		return errors.New("nil reader")
	}

	buf := make([]byte, MTU) // todo: pool
	n, err := r.Read(buf)
	if err != nil {
		return err
	}
	buf = buf[:n]
	ptr := 0

	for _, d := range data {
		slashIdx := nextSlash(buf[ptr:])
		if slashIdx == -1 {
			return ErrMessageInvalid
		}

		switch e := d.(type) {
		case *int32:
			parseInt, err := strconv.ParseInt(string(buf[ptr:ptr+slashIdx]), 10, 32)
			if err != nil {
				return err
			}
			*e = int32(parseInt)
		case *[]byte:
			curBuf := buf[ptr : ptr+slashIdx]
			bytesVal := make([]byte, 0, len(curBuf))
			for idx, val := range curBuf {
				if val == '\\' && (idx > 0 && curBuf[idx-1] != '\\') {
					continue
				}
				//if idx > 0 && curBuf[idx-1] == '\\' {
				//	switch val {
				//	case 'n':
				//		val = '\n'
				//	}
				//}
				bytesVal = append(bytesVal, val)
			}
			*e = bytesVal
		default:
			return errors.New("unimplemented type")
		}
		ptr += slashIdx + 1
	}

	if ptr < len(buf) {
		return ErrMessageInvalid
	}

	return nil
}

func nextSlash(buf []byte) int {
	for idx, val := range buf {
		if val == '/' {
			if idx > 0 && buf[idx-1] == '\\' { // handle forward slash as a data
				continue
			}
			return idx
		}
	}
	return -1
}
