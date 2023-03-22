package main

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		wantErr bool
	}{
		{
			name:    "error",
			message: &Error{Message: "bad"},
		},
		{
			name:    "error",
			message: &Error{Message: "illegal msg"},
		},
		{
			name:    "error - invalid string",
			message: &Error{Message: "illegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msgillegal msg"},
			wantErr: true,
		},
		{
			name: "plate",
			message: &Plate{
				Plate:     "UNIX",
				Timestamp: 1000,
			},
		},
		{
			name: "plate",
			message: &Plate{
				Plate:     "RE05BKG",
				Timestamp: 123456,
			},
		},
		{
			name: "ticket",
			message: &Ticket{
				Plate:      "UNIX",
				Road:       66,
				Mile1:      1000,
				Timestamp1: 123456,
				Mile2:      110,
				Timestamp2: 123816,
				Speed:      10000,
			},
		},
		{
			name: "ticket",
			message: &Ticket{
				Plate:      "RE05BKG",
				Road:       368,
				Mile1:      1234,
				Timestamp1: 1000000,
				Mile2:      1235,
				Timestamp2: 1000060,
				Speed:      6000,
			},
		},
		{
			name:    "want-heart-beat",
			message: &WantHeartbeat{Interval: 10},
		},
		{
			name:    "want-heart-beat",
			message: &WantHeartbeat{Interval: 1243},
		},
		{
			name:    "heart-beat",
			message: &Heartbeat{},
		},
		{
			name: "i-am-camera",
			message: &IAmCamera{
				Road:  66,
				Mile:  100,
				Limit: 60,
			},
		},
		{
			name: "i-am-camera",
			message: &IAmCamera{
				Road:  368,
				Mile:  1234,
				Limit: 40,
			},
		},
		{
			name: "i-am-dispatcher",
			message: &IAmDispatcher{
				Roads: []uint16{
					66,
				},
			},
		},
		{
			name: "i-am-dispatcher",
			message: &IAmDispatcher{
				Roads: []uint16{
					66,
					368,
					5000,
				},
			},
		},
		{
			name: "i-am-dispatcher - invalid array length",
			message: &IAmDispatcher{
				Roads: func() []uint16 {
					return make([]uint16, 256)
				}(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.message == nil {
				return
			}
			buf := &bytes.Buffer{}
			err := tt.message.Marshal(buf)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Error(err)
			}
			message, err := UnmarshalMessage(buf)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Error(err)
			}
			if !reflect.DeepEqual(message, tt.message) {
				t.Errorf("not equal, %#v %#v", message, tt.message)
			}
		})
	}
}
