package proto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataType(t *testing.T) {
	tests := []struct {
		name                string
		datatype            DataType
		expectedVal         []byte
		expectedPreChecksum byte
		wantErr             bool
	}{
		{
			name:                "u32",
			datatype:            NewU32(32),
			expectedVal:         []byte("\x00\x00\x00\x20"),
			expectedPreChecksum: 0x20,
		},
		{
			name:                "u32",
			datatype:            NewU32(4677),
			expectedVal:         []byte("\x00\x00\x12\x45"),
			expectedPreChecksum: 0x57,
		},
		{
			name:                "u32",
			datatype:            NewU32(2796139879),
			expectedVal:         []byte("\xa6\xa9\xb5\x67"),
			expectedPreChecksum: 0x6B,
		},
		{
			name:                "string",
			datatype:            NewString(""),
			expectedVal:         []byte("\x00\x00\x00\x00"),
			expectedPreChecksum: 0x00,
		},
		{
			name:                "string",
			datatype:            NewString("foo"),
			expectedVal:         []byte("\x00\x00\x00\x03\x66\x6f\x6f"),
			expectedPreChecksum: 0x47,
		},
		{
			name:                "string",
			datatype:            NewString("Elbereth"),
			expectedVal:         []byte("\x00\x00\x00\x08\x45\x6C\x62\x65\x72\x65\x74\x68"),
			expectedPreChecksum: 0x33,
		},
		{
			name:                "array(uint32)",
			datatype:            NewArrayWithData(NewU32(32), NewU32(4677)),
			expectedVal:         []byte("\x00\x00\x00\x02\x00\x00\x00\x20\x00\x00\x12\x45"),
			expectedPreChecksum: 0x79,
		},
		{
			name:                "array(string)",
			datatype:            NewArrayWithData(NewString("Elbereth"), NewString("")),
			expectedVal:         []byte("\x00\x00\x00\x02\x00\x00\x00\x08\x45\x6C\x62\x65\x72\x65\x74\x68\x00\x00\x00\x00"),
			expectedPreChecksum: 0x35,
		},
		{
			name:                "message_hello",
			datatype:            &MessageHello{Protocol: "pestcontrol", Version: 1},
			expectedVal:         []byte("\x50\x00\x00\x00\x19\x00\x00\x00\x0b\x70\x65\x73\x74\x63\x6f\x6e\x74\x72\x6f\x6c\x00\x00\x00\x01\xce"),
			expectedPreChecksum: 0x00,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			preChecksum, err := tt.datatype.Marshal(buf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPreChecksum, preChecksum)
			b := buf.Bytes()
			assert.Equal(t, tt.expectedVal, b)

			tt.datatype.Reset()
			n, preChecksum, err := tt.datatype.Unmarshal(buf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPreChecksum, preChecksum)
			assert.Equal(t, len(tt.expectedVal), n)
			assert.Equal(t, uint32(len(tt.expectedVal)), tt.datatype.BytesLength())
			buf.Reset()
			preChecksum, err = tt.datatype.Marshal(buf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPreChecksum, preChecksum)
			b = buf.Bytes()
			assert.Equal(t, tt.expectedVal, b)
		})
	}
}
