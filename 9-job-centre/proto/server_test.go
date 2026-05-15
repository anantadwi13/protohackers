package proto

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_connHandler_nextRequestBuffer(t *testing.T) {
	type args struct {
		bufSize int
		reader  io.Reader
	}
	tests := []struct {
		name         string
		args         args
		wantCurData  [][]byte
		wantNextData []byte
		wantRestData []byte
		wantErr      bool
	}{
		{
			name: "success",
			args: args{
				bufSize: 5,
				reader:  strings.NewReader("1234567890abcde\nfghijklmno"),
			},
			wantCurData: [][]byte{
				[]byte("12345"),
				[]byte("67890"),
				[]byte("abcde"),
			},
			wantNextData: []byte("fghi"),
			wantRestData: []byte("jklmno"),
			wantErr:      false,
		},
		{
			name: "success",
			args: args{
				bufSize: 5,
				reader:  strings.NewReader("1234567890abcdef\nghijklmno"),
			},
			wantCurData: [][]byte{
				[]byte("12345"),
				[]byte("67890"),
				[]byte("abcde"),
				[]byte("f"),
			},
			wantNextData: []byte("ghi"),
			wantRestData: []byte("jklmno"),
			wantErr:      false,
		},
		{
			name: "success",
			args: args{
				bufSize: 5,
				reader:  strings.NewReader("1234567890abc\ndefghijklmno"),
			},
			wantCurData: [][]byte{
				[]byte("12345"),
				[]byte("67890"),
				[]byte("abc"),
			},
			wantNextData: []byte("d"),
			wantRestData: []byte("efghijklmno"),
			wantErr:      false,
		},
		{
			name: "success",
			args: args{
				bufSize: 5,
				reader:  strings.NewReader("1234567890abcd\nefghijklmno"),
			},
			wantCurData: [][]byte{
				[]byte("12345"),
				[]byte("67890"),
				[]byte("abcd"),
			},
			wantNextData: nil,
			wantRestData: []byte("efghijklmno"),
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			curReqBuf := &requestBuffer{}
			got, gotRestBuf, err := nextRequestBuffer(context.Background(), tt.args.bufSize, tt.args.reader, curReqBuf, nil)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			restData, err := io.ReadAll(tt.args.reader)
			assert.NoError(t, err)

			assert.NoError(t, err)
			assert.EqualValues(t, tt.wantCurData, curReqBuf.data)
			assert.EqualValues(t, tt.wantRestData, restData)
			assert.EqualValues(t, tt.wantNextData, gotRestBuf)
			assert.Nil(t, got.data)
		})
	}
}

func TestRequestBuffer(t *testing.T) {
	reqBuf := &requestBuffer{
		data: [][]byte{
			[]byte("abcdef"),
			[]byte("ghijkl"),
			[]byte("123"),
		},
		pos: 0,
	}

	data, err := io.ReadAll(reqBuf)
	assert.NoError(t, err)
	assert.EqualValues(t, []byte("abcdefghijkl123"), data)

	seek, err := reqBuf.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	assert.EqualValues(t, 15, seek)

	buf := make([]byte, 8)
	n, err := reqBuf.Read(buf)
	assert.NoError(t, err)
	assert.EqualValues(t, 8, n)
	assert.EqualValues(t, []byte("abcdefgh"), buf[:n])

	buf = make([]byte, 8)
	n, err = reqBuf.Read(buf)
	assert.NoError(t, err)
	assert.EqualValues(t, 7, n)
	assert.EqualValues(t, []byte("ijkl123"), buf[:n])
}
