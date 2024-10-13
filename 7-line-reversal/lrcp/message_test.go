package lrcp

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestMarshalUnmarshalMessage(t *testing.T) {
	type args struct {
		raw string
	}
	tests := []struct {
		name    string
		args    args
		wantMsg Message
		wantErr bool
	}{
		{
			name:    "success connect",
			args:    args{"/connect/1234567/"},
			wantMsg: &MessageConnect{sessionNumber: 1234567},
			wantErr: false,
		},
		{
			name: "success data",
			args: args{"/data/1234567/789/foo\\/bar\\\\baz\nnewline/"},
			wantMsg: &MessageData{
				sessionNumber: 1234567,
				Pos:           789,
				Data:          []byte("foo/bar\\baz\nnewline"),
			},
			wantErr: false,
		},
		{
			name: "success data",
			args: args{"/data/12345/0/hello\n/"},
			wantMsg: &MessageData{
				sessionNumber: 12345,
				Pos:           0,
				Data:          []byte("hello\n"),
			},
			wantErr: false,
		},
		{
			name:    "success ack",
			args:    args{"/ack/1234567/100/"},
			wantMsg: &MessageAck{sessionNumber: 1234567, Length: 100},
			wantErr: false,
		},
		{
			name:    "success close",
			args:    args{"/close/1234567/"},
			wantMsg: &MessageClose{sessionNumber: 1234567},
			wantErr: false,
		},
		{
			name:    "error",
			args:    args{"/close/1234567"},
			wantMsg: &MessageUnknown{},
			wantErr: true,
		},
		{
			name:    "error not ended by /",
			args:    args{"/close/1234567/ "},
			wantMsg: &MessageUnknown{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// unmarshal
			gotMsg, err := UnmarshalMessage(strings.NewReader(tt.args.raw))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("UnmarshalMessage() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
			if tt.wantErr {
				return
			}
			// marshal
			buf := &bytes.Buffer{}
			err = gotMsg.Marshal(buf)
			if err != nil {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(buf.String(), tt.args.raw) {
				t.Errorf("UnmarshalMessage() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}
