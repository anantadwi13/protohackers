package proto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataType(t *testing.T) {
	tests := []struct {
		name        string
		datatype    DataType
		expectedVal []byte
		wantErr     bool
	}{
		{
			name:        "u32",
			datatype:    NewU32(32),
			expectedVal: []byte("\x00\x00\x00\x20"),
		},
		{
			name:        "u32",
			datatype:    NewU32(4677),
			expectedVal: []byte("\x00\x00\x12\x45"),
		},
		{
			name:        "u32",
			datatype:    NewU32(2796139879),
			expectedVal: []byte("\xa6\xa9\xb5\x67"),
		},
		{
			name:        "string",
			datatype:    NewString(""),
			expectedVal: []byte("\x00\x00\x00\x00"),
		},
		{
			name:        "string",
			datatype:    NewString("foo"),
			expectedVal: []byte("\x00\x00\x00\x03\x66\x6f\x6f"),
		},
		{
			name:        "string",
			datatype:    NewString("Elbereth"),
			expectedVal: []byte("\x00\x00\x00\x08\x45\x6C\x62\x65\x72\x65\x74\x68"),
		},
		{
			name:        "array(uint32)",
			datatype:    NewArrayWithData(NewU32(32), NewU32(4677)),
			expectedVal: []byte("\x00\x00\x00\x02\x00\x00\x00\x20\x00\x00\x12\x45"),
		},
		{
			name:        "array(string)",
			datatype:    NewArrayWithData(NewString("Elbereth"), NewString("")),
			expectedVal: []byte("\x00\x00\x00\x02\x00\x00\x00\x08\x45\x6C\x62\x65\x72\x65\x74\x68\x00\x00\x00\x00"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			err := tt.datatype.Marshal(buf)
			assert.NoError(t, err)
			b := buf.Bytes()
			assert.Equal(t, b, tt.expectedVal)

			tt.datatype.Reset()
			err = tt.datatype.Unmarshal(buf)
			assert.NoError(t, err)
			buf.Reset()
			err = tt.datatype.Marshal(buf)
			assert.NoError(t, err)
			b = buf.Bytes()
			assert.Equal(t, b, tt.expectedVal)
		})
	}
}
