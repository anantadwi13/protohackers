package main

import (
	"encoding/binary"
	"errors"
	"io"
)

const MaxByte = 1<<8 - 1

type Message interface {
	Marshal(writer io.Writer) error
	Unmarshaler(reader io.Reader) error
}

func UnmarshalMessage(reader io.Reader) (Message, error) {
	msgType := make([]byte, 1)
	n, err := io.ReadFull(reader, msgType)
	if err != nil {
		return nil, err
	}
	if n != len(msgType) {
		return nil, errors.New("invalid message")
	}

	var msg Message
	switch msgType[0] {
	case 0x10:
		msg = &Error{}
	case 0x20:
		msg = &Plate{}
	case 0x21:
		msg = &Ticket{}
	case 0x40:
		msg = &WantHeartbeat{}
	case 0x41:
		msg = &Heartbeat{}
	case 0x80:
		msg = &IAmCamera{}
	case 0x81:
		msg = &IAmDispatcher{}
	default:
		return &Unknown{}, nil
	}

	err = msg.Unmarshaler(reader)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

type Unknown struct {
}

func (u Unknown) Marshal(writer io.Writer) error {
	return nil
}

func (u *Unknown) Unmarshaler(reader io.Reader) error {
	return nil
}

type Error struct {
	Message string
}

func (e Error) Marshal(writer io.Writer) error {
	return constructPacketData(
		writer,
		uint8(0x10), // message type
		e.Message,
	)
}

func (e *Error) Unmarshaler(reader io.Reader) error {
	return extractPacketData(
		reader,
		&e.Message,
	)
}

type Plate struct {
	Plate     string
	Timestamp uint32
}

func (p Plate) Marshal(writer io.Writer) error {
	return constructPacketData(
		writer,
		uint8(0x20), // message type
		p.Plate,
		p.Timestamp,
	)
}

func (p *Plate) Unmarshaler(reader io.Reader) error {
	return extractPacketData(
		reader,
		&p.Plate,
		&p.Timestamp,
	)
}

type Ticket struct {
	Plate      string
	Road       uint16
	Mile1      uint16
	Timestamp1 uint32
	Mile2      uint16
	Timestamp2 uint32
	Speed      uint16 // 100x miles per hour
}

func (t Ticket) Marshal(writer io.Writer) error {
	return constructPacketData(
		writer,
		uint8(0x21), // message type
		t.Plate,
		t.Road,
		t.Mile1,
		t.Timestamp1,
		t.Mile2,
		t.Timestamp2,
		t.Speed,
	)
}

func (t *Ticket) Unmarshaler(reader io.Reader) error {
	return extractPacketData(
		reader,
		&t.Plate,
		&t.Road,
		&t.Mile1,
		&t.Timestamp1,
		&t.Mile2,
		&t.Timestamp2,
		&t.Speed,
	)
}

type WantHeartbeat struct {
	Interval uint32 // deciseconds
}

func (w WantHeartbeat) Marshal(writer io.Writer) error {
	return constructPacketData(
		writer,
		uint8(0x40), // message type
		w.Interval,
	)
}

func (w *WantHeartbeat) Unmarshaler(reader io.Reader) error {
	return extractPacketData(
		reader,
		&w.Interval,
	)
}

type Heartbeat struct {
}

func (h Heartbeat) Marshal(writer io.Writer) error {
	return constructPacketData(
		writer,
		uint8(0x41), // message type
	)
}

func (h *Heartbeat) Unmarshaler(reader io.Reader) error {
	return extractPacketData(
		reader,
	)
}

type IAmCamera struct {
	Road  uint16
	Mile  uint16
	Limit uint16 // miles per hour
}

func (c IAmCamera) Marshal(writer io.Writer) error {
	return constructPacketData(
		writer,
		uint8(0x80), // message type
		c.Road,
		c.Mile,
		c.Limit,
	)
}

func (c *IAmCamera) Unmarshaler(reader io.Reader) error {
	return extractPacketData(
		reader,
		&c.Road,
		&c.Mile,
		&c.Limit,
	)
}

type IAmDispatcher struct {
	Roads []uint16
}

func (d IAmDispatcher) Marshal(writer io.Writer) error {
	return constructPacketData(
		writer,
		uint8(0x81), // message type
		d.Roads,
	)
}

func (d *IAmDispatcher) Unmarshaler(reader io.Reader) error {
	return extractPacketData(
		reader,
		&d.Roads,
	)
}

func constructPacketData(w io.Writer, data ...any) error {
	if w == nil {
		return errors.New("nil writer")
	}

	for _, d := range data {
		switch e := d.(type) {
		case uint8, uint16, uint32:
			err := binary.Write(w, binary.BigEndian, e)
			if err != nil {
				return err
			}
		case string:
			if len(e) > MaxByte {
				return errors.New("invalid string length")
			}
			n, err := w.Write([]byte{byte(len(e))})
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("invalid write packet length")
			}
			n, err = w.Write([]byte(e))
			if err != nil {
				return err
			}
		case []uint16:
			if len(e) > MaxByte {
				return errors.New("invalid array length")
			}
			n, err := w.Write([]byte{byte(len(e))})
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("invalid write packet length")
			}
			for _, u := range e {
				err = binary.Write(w, binary.BigEndian, u)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func extractPacketData(r io.Reader, data ...any) error {
	if r == nil {
		return errors.New("empty reader")
	}

	for _, d := range data {
		switch e := d.(type) {
		case *uint8, *uint16, *uint32:
			err := binary.Read(r, binary.BigEndian, e)
			if err != nil {
				return err
			}
		case *string:
			lengthBuf := make([]byte, 1)
			n, err := io.ReadFull(r, lengthBuf)
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("invalid packet length")
			}
			strBuf := make([]byte, lengthBuf[0])
			n, err = io.ReadFull(r, strBuf)
			if err != nil {
				return err
			}
			if n != len(strBuf) {
				return errors.New("invalid string length")
			}
			*e = string(strBuf)
		case *[]uint16:
			lengthBuf := make([]byte, 1)
			n, err := io.ReadFull(r, lengthBuf)
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("invalid packet length")
			}
			arr := make([]uint16, lengthBuf[0])
			for i := range arr {
				err = binary.Read(r, binary.BigEndian, &arr[i])
				if err != nil {
					return err
				}
			}
			*e = arr
		}
	}

	return nil
}
