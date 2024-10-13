package lrcp

import (
	"context"
	"reflect"
	"testing"
)

func Test_sessionUDP_readNewLineFromMessage(t *testing.T) {
	type fields struct {
		lenTotalIncoming int32
		incomingChan     chan []byte
		incomingBuf      []byte
	}
	type args struct {
		m *MessageData
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    [][]byte
		wantBuf []byte
	}{
		{
			name: "success",
			fields: fields{
				incomingChan: make(chan []byte),
			},
			args: args{
				m: &MessageData{
					sessionNumber: 0,
					Pos:           0,
					Data:          []byte("first line\nsecond line\n"),
				},
			},
			want: [][]byte{
				[]byte("first line"),
				[]byte("second line"),
			},
		},
		{
			name: "success",
			fields: fields{
				incomingChan: make(chan []byte),
				incomingBuf: func() []byte {
					buf := make([]byte, 0, MTU)
					buf = append(buf, "12345"...)
					return buf
				}(),
				lenTotalIncoming: 10,
			},
			args: args{
				m: &MessageData{
					sessionNumber: 0,
					Pos:           10,
					Data:          []byte("first line\nsecond line\nnice"),
				},
			},
			want: [][]byte{
				[]byte("12345first line"),
				[]byte("second line"),
			},
			wantBuf: []byte("nice"),
		},
		{
			name: "success",
			fields: fields{
				incomingChan: make(chan []byte),
				incomingBuf: func() []byte {
					buf := make([]byte, 0, MTU)
					buf = append(buf, "12345"...)
					return buf
				}(),
				lenTotalIncoming: 10,
			},
			args: args{
				m: &MessageData{
					sessionNumber: 0,
					Pos:           10,
					Data:          []byte("first line\nsecond line\nnice\nnext"),
				},
			},
			want: [][]byte{
				[]byte("12345first line"),
				[]byte("second line"),
				[]byte("nice"),
			},
			wantBuf: []byte("next"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sessionUDP{
				ctx:              context.Background(),
				lenTotalIncoming: tt.fields.lenTotalIncoming,
				incomingChan:     tt.fields.incomingChan,
				incomingBuf:      tt.fields.incomingBuf,
			}
			go func() {
				defer close(s.incomingChan)
				s.readNewLineFromMessage(tt.args.m)
			}()
			var got [][]byte
			for bytes := range s.incomingChan {
				got = append(got, bytes)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readNewLineFromMessage() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(s.incomingBuf, tt.wantBuf) {
				t.Errorf("readNewLineFromMessage() incomingBuf = %v, wantBuf %v", got, tt.want)
			}
		})
	}
}
