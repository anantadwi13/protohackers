package lrcp

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_message_unmarshal_marshal(t *testing.T) {
	type want struct {
		MessageType messageType
		SessionId   int32
		Length      int32
		Data        []byte
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "success connect",
			want: want{
				MessageType: messageTypeConnect,
				SessionId:   12345,
				Length:      0,
				Data:        nil,
			},
			args:    args{data: []byte(`/connect/12345/`)},
			wantErr: false,
		},
		{
			name: "success ack",
			want: want{
				MessageType: messageTypeAck,
				SessionId:   12345,
				Length:      0,
				Data:        nil,
			},
			args:    args{data: []byte(`/ack/12345/0/`)},
			wantErr: false,
		},
		{
			name: "success data",
			want: want{
				MessageType: messageTypeData,
				SessionId:   12345,
				Length:      0,
				Data:        []byte("hello\n"),
			},
			args:    args{data: []byte(`/data/12345/0/hello\n/`)},
			wantErr: false,
		},
		{
			name: "success data",
			want: want{
				MessageType: messageTypeData,
				SessionId:   12345,
				Length:      0,
				Data:        []byte("hello"),
			},
			args:    args{data: []byte(`/data/12345/0/hello/`)},
			wantErr: false,
		},
		{
			name: "success data",
			want: want{
				MessageType: messageTypeData,
				SessionId:   12345,
				Length:      6,
				Data:        []byte("Hello,\\/ world!\n"),
			},
			args:    args{data: []byte(`/data/12345/6/Hello,\\\/ world!\n/`)},
			wantErr: false,
		},
		{
			name: "success ack",
			want: want{
				MessageType: messageTypeAck,
				SessionId:   12345,
				Length:      6,
				Data:        nil,
			},
			args:    args{data: []byte(`/ack/12345/6/`)},
			wantErr: false,
		},
		{
			name: "success close",
			want: want{
				MessageType: messageTypeClose,
				SessionId:   12345,
				Length:      0,
				Data:        nil,
			},
			args:    args{data: []byte(`/close/12345/`)},
			wantErr: false,
		},
		{
			name:    "invalid message",
			want:    want{},
			args:    args{data: []byte(`/close/12345//`)},
			wantErr: true,
		},
		{
			name:    "invalid message",
			want:    want{},
			args:    args{data: []byte(`/data/12345/6/Hello, world!/\n/`)},
			wantErr: true,
		},
		{
			name:    "invalid message",
			want:    want{},
			args:    args{data: []byte(`/data/12345/6//Hello, world!\n/`)},
			wantErr: true,
		},
		{
			name:    "invalid message",
			want:    want{},
			args:    args{data: []byte(`/data/2147483648/6/Hello, world!\n/`)},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantMsg := &message{
				MessageType: tt.want.MessageType,
				SessionId:   tt.want.SessionId,
				Length:      tt.want.Length,
				Data:        tt.want.Data,
			}
			p := &message{}
			bufInput := make([]byte, maxMessageSize)
			n := copy(bufInput, tt.args.data)
			err := p.unmarshal(bufInput[:n])
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, wantMsg, p)
			bufOutput := make([]byte, 0, maxMessageSize)
			n, _, err = p.marshal(bufOutput)
			assert.NoError(t, err)
			assert.Equal(t, tt.args.data, bufOutput[:n])
		})
	}
}

func Test_message_marshal(t *testing.T) {
	defaultMaxMessageSize := maxMessageSize

	type args struct {
		MessageType messageType
		SessionId   int32
		Length      int32
		Data        []byte
	}
	tests := []struct {
		name          string
		args          args
		mock          func(t *testing.T)
		mockDefer     func(t *testing.T)
		wantData      []byte
		wantDataWrite int
		wantErr       assert.ErrorAssertionFunc
	}{
		{
			name: "success",
			args: args{
				MessageType: messageTypeData,
				SessionId:   91234,
				Length:      0,
				Data:        []byte("hello\n"),
			},
			wantData:      []byte(`/data/91234/0/hello\n/`),
			wantDataWrite: 6,
			wantErr:       assert.NoError,
		},
		{
			name: "success",
			args: args{
				MessageType: messageTypeData,
				SessionId:   91234,
				Length:      0,
				Data:        []byte("hello\n"),
			},
			mock: func(t *testing.T) {
				maxMessageSize = 18
			},
			mockDefer: func(t *testing.T) {
				maxMessageSize = defaultMaxMessageSize
			},
			wantData:      []byte(`/data/91234/0/hel/`),
			wantDataWrite: 3,
			wantErr:       assert.NoError,
		},
		{
			name: "success",
			args: args{
				MessageType: messageTypeData,
				SessionId:   91234,
				Length:      0,
				Data:        []byte("hello\n"),
			},
			mock: func(t *testing.T) {
				maxMessageSize = 21
			},
			mockDefer: func(t *testing.T) {
				maxMessageSize = defaultMaxMessageSize
			},
			wantData:      []byte(`/data/91234/0/hello/`),
			wantDataWrite: 5,
			wantErr:       assert.NoError,
		},
		{
			name: "success",
			args: args{
				MessageType: messageTypeData,
				SessionId:   91234,
				Length:      0,
				Data:        []byte("hello\n/"),
			},
			mock: func(t *testing.T) {
				maxMessageSize = 22
			},
			mockDefer: func(t *testing.T) {
				maxMessageSize = defaultMaxMessageSize
			},
			wantData:      []byte(`/data/91234/0/hello\n/`),
			wantDataWrite: 6,
			wantErr:       assert.NoError,
		},
		{
			name: "success",
			args: args{
				MessageType: messageTypeData,
				SessionId:   91234,
				Length:      0,
				Data:        []byte("hello\n/"),
			},
			mock: func(t *testing.T) {
				maxMessageSize = 24
			},
			mockDefer: func(t *testing.T) {
				maxMessageSize = defaultMaxMessageSize
			},
			wantData:      []byte(`/data/91234/0/hello\n\//`),
			wantDataWrite: 7,
			wantErr:       assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mock != nil {
				tt.mock(t)
			}
			if tt.mockDefer != nil {
				defer tt.mockDefer(t)
			}

			p := &message{
				MessageType: tt.args.MessageType,
				SessionId:   tt.args.SessionId,
				Length:      tt.args.Length,
				Data:        tt.args.Data,
			}
			buf := make([]byte, 0, maxMessageSize)
			got, got1, err := p.marshal(buf)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.wantData, buf[:got])
			assert.Equal(t, len(tt.wantData), got)
			assert.Equal(t, tt.wantDataWrite, got1)
		})
	}
}
