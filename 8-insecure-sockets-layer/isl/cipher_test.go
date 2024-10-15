package isl

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCipherSpec(t *testing.T) {
	type args struct {
		plainText []byte
		ops       []CipherOperation
	}
	tests := []struct {
		name       string
		args       args
		cipherText []byte
	}{
		{
			name: "success",
			args: args{
				plainText: []byte("\x12\x34\x56\x78"),
				ops:       []CipherOperation{&CipherOperationReverseBits{}},
			},
			cipherText: []byte("\x48\x2c\x6a\x1e"),
		},
		{
			name: "success xor(1),reversebits",
			args: args{
				plainText: []byte("hello"),
				ops: []CipherOperation{
					&CipherOperationXor{N: 0x01},
					&CipherOperationReverseBits{},
				},
			},
			cipherText: []byte("\x96\x26\xb6\xb6\x76"),
		},
		{
			name: "success addpos,addpos",
			args: args{
				plainText: []byte("hello"),
				ops: []CipherOperation{
					&CipherOperationAddPos{},
					&CipherOperationAddPos{},
				},
			},
			cipherText: []byte("\x68\x67\x70\x72\x77"),
		},
		{
			name: "success xor(123),addpos,reversebits",
			args: args{
				plainText: []byte(
					"5x car\n" +
						"3x rat\n",
				),
				ops: []CipherOperation{
					&CipherOperationXor{N: 0x7b},
					&CipherOperationAddPos{},
					&CipherOperationReverseBits{},
				},
			},
			cipherText: []byte(
				"\x72\x20\xba\xd8\x78\x70\xee" +
					"\xf2\xd0\x26\xc8\xa4\xd8\x7e",
			),
		},
		{
			name: "success xor(123),addpos,reversebits",
			args: args{
				plainText: []byte(
					"4x dog,5x car\n" +
						"3x rat,2x cat\n",
				),
				ops: []CipherOperation{
					&CipherOperationXor{N: 0x7b},
					&CipherOperationAddPos{},
					&CipherOperationReverseBits{},
				},
			},
			cipherText: []byte(
				"\xf2\x20\xba\x44\x18\x84\xba\xaa\xd0\x26\x44\xa4\xa8\x7e" +
					"\x6a\x48\xd6\x58\x34\x44\xd6\x7a\x98\x4e\x0c\xcc\x94\x31",
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cipherTextBuf := bytes.NewBuffer(nil)
			spec := &CipherSpec{
				Operations: tt.args.ops,
			}

			reader := spec.NewReader(cipherTextBuf)
			writer := spec.NewWriter(cipherTextBuf)
			assert.NotNil(t, reader)
			assert.NotNil(t, writer)

			n, err := writer.Write(tt.args.plainText)
			assert.NoError(t, err)
			assert.EqualValues(t, len(tt.args.plainText), n)

			assert.EqualValues(t, tt.cipherText, cipherTextBuf.Bytes())

			gotPlainText := make([]byte, len(tt.args.plainText))
			n, err = reader.Read(gotPlainText)
			assert.NoError(t, err)
			assert.EqualValues(t, len(gotPlainText), n)
		})
	}
}
