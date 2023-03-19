package main

import (
	"errors"
)

type Message interface {
	_typeMessage()
}

func UnmarshalMessage(raw []byte) (Message, error) {
	if len(raw) != 9 {
		return nil, errors.New("invalid message")
	}

	switch raw[0] {
	case 'I':
		timestamp, err := bytesToInt32(raw[1:5])
		if err != nil {
			return nil, err
		}
		price, err := bytesToInt32(raw[5:])
		if err != nil {
			return nil, err
		}
		return &MessageInsert{
			Timestamp: timestamp,
			Price:     price,
		}, nil
	case 'Q':
		minTime, err := bytesToInt32(raw[1:5])
		if err != nil {
			return nil, err
		}
		maxTime, err := bytesToInt32(raw[5:])
		if err != nil {
			return nil, err
		}
		return &MessageQuery{
			MinTime: minTime,
			MaxTime: maxTime,
		}, nil
	default:
		return nil, errors.New("unknown message type")
	}
}

type MessageInsert struct {
	Timestamp int32
	Price     int32
}

func (m *MessageInsert) _typeMessage() {
}

type MessageQuery struct {
	MinTime int32
	MaxTime int32
}

func (m *MessageQuery) _typeMessage() {
}

type Result struct {
	MeanPrice int32
}

func (r *Result) MarshalBytes() ([]byte, error) {
	return int32ToBytes(r.MeanPrice)
}
