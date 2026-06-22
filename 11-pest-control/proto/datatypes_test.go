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
			name:                "array(uint32) empty",
			datatype:            NewArray[*U32](),
			expectedVal:         []byte("\x00\x00\x00\x00"),
			expectedPreChecksum: 0x00,
		},
		{
			name:                "array(uint32)",
			datatype:            NewArray(NewU32(32), NewU32(4677)),
			expectedVal:         []byte("\x00\x00\x00\x02\x00\x00\x00\x20\x00\x00\x12\x45"),
			expectedPreChecksum: 0x79,
		},
		{
			name:                "array(string) empty",
			datatype:            NewArray[*String](),
			expectedVal:         []byte("\x00\x00\x00\x00"),
			expectedPreChecksum: 0x00,
		},
		{
			name:                "array(string)",
			datatype:            NewArray(NewString("Elbereth"), NewString("")),
			expectedVal:         []byte("\x00\x00\x00\x02\x00\x00\x00\x08\x45\x6C\x62\x65\x72\x65\x74\x68\x00\x00\x00\x00"),
			expectedPreChecksum: 0x35,
		},
		{
			name:                "siteId",
			datatype:            new(SiteId(12345)),
			expectedVal:         []byte("\x00\x00\x30\x39"),
			expectedPreChecksum: 0x69,
		},
		{
			name:                "policyId",
			datatype:            new(PolicyId(123)),
			expectedVal:         []byte("\x00\x00\x00\x7b"),
			expectedPreChecksum: 0x7b,
		},
		{
			name:                "species",
			datatype:            new(Species("dog")),
			expectedVal:         []byte("\x00\x00\x00\x03\x64\x6f\x67"),
			expectedPreChecksum: 0x3d,
		},
		{
			name:                "policy_action",
			datatype:            new(PolicyActionCull),
			expectedVal:         []byte("\x90"),
			expectedPreChecksum: 0x90,
		},
		{
			name: "target_populations_population",
			datatype: new(TargetPopulationsPopulation{
				Species: "dog",
				Min:     1,
				Max:     3,
			}),
			expectedVal: []byte("\x00\x00\x00\x03" +
				"\x64\x6f\x67" +
				"\x00\x00\x00\x01" +
				"\x00\x00\x00\x03"),
			expectedPreChecksum: 0x41,
		},
		{
			name: "site_visit_population",
			datatype: new(SiteVisitPopulation{
				Species: "dog",
				Count:   3,
			}),
			expectedVal: []byte("\x00\x00\x00\x03" +
				"\x64\x6f\x67" +
				"\x00\x00\x00\x03"),
			expectedPreChecksum: 0x40,
		},
		{
			name:     "message_hello",
			datatype: &MessageHello{Protocol: "pestcontrol", Version: 1},
			expectedVal: []byte("\x50" +
				"\x00\x00\x00\x19" +
				"\x00\x00\x00\x0b" +
				"\x70\x65\x73\x74" +
				"\x63\x6f\x6e\x74" +
				"\x72\x6f\x6c" +
				"\x00\x00\x00\x01" +
				"\xce"),
			expectedPreChecksum: 0x00,
		},
		{
			name:     "message_error",
			datatype: &MessageError{Message: "bad"},
			expectedVal: []byte("\x51" +
				"\x00\x00\x00\x0d" +
				"\x00\x00\x00\x03" +
				"\x62\x61\x64" +
				"\x78"),
			expectedPreChecksum: 0x00,
		},
		{
			name:     "message_ok",
			datatype: &MessageOk{},
			expectedVal: []byte("\x52" +
				"\x00\x00\x00\x06" +
				"\xa8"),
			expectedPreChecksum: 0x00,
		},
		{
			name:     "message_dial_authority",
			datatype: &MessageDialAuthority{Site: 12345},
			expectedVal: []byte("\x53" +
				"\x00\x00\x00\x0a" +
				"\x00\x00\x30\x39" +
				"\x3a"),
			expectedPreChecksum: 0x00,
		},
		{
			name: "message_target_populations",
			datatype: &MessageTargetPopulations{
				Site: 12345,
				Populations: *NewArray(
					&TargetPopulationsPopulation{
						Species: "dog",
						Min:     1,
						Max:     3,
					},
					&TargetPopulationsPopulation{
						Species: "rat",
						Min:     0,
						Max:     10,
					},
				),
			},
			expectedVal: []byte("\x54" +
				"\x00\x00\x00\x2c" +
				"\x00\x00\x30\x39" +
				"\x00\x00\x00\x02" +
				"\x00\x00\x00\x03" +
				"\x64\x6f\x67" +
				"\x00\x00\x00\x01" +
				"\x00\x00\x00\x03" +
				"\x00\x00\x00\x03" +
				"\x72\x61\x74" +
				"\x00\x00\x00\x00" +
				"\x00\x00\x00\x0a" +
				"\x80"),
			expectedPreChecksum: 0x00,
		},
		{
			name: "message_create_policy",
			datatype: &MessageCreatePolicy{
				Species: "dog",
				Action:  PolicyActionConserve,
			},
			expectedVal: []byte("\x55" +
				"\x00\x00\x00\x0e" +
				"\x00\x00\x00\x03" +
				"\x64\x6f\x67" +
				"\xa0" +
				"\xc0"),
			expectedPreChecksum: 0x00,
		},
		{
			name: "message_delete_policy",
			datatype: &MessageDeletePolicy{
				Policy: PolicyId(123),
			},
			expectedVal: []byte("\x56" +
				"\x00\x00\x00\x0a" +
				"\x00\x00\x00\x7b" +
				"\x25"),
			expectedPreChecksum: 0x00,
		},
		{
			name: "message_policy_result",
			datatype: &MessagePolicyResult{
				Policy: PolicyId(123),
			},
			expectedVal: []byte("\x57" +
				"\x00\x00\x00\x0a" +
				"\x00\x00\x00\x7b" +
				"\x24"),
			expectedPreChecksum: 0x00,
		},
		{
			name: "message_site_visit",
			datatype: &MessageSiteVisit{
				Site: 12345,
				Populations: *NewArray(
					&SiteVisitPopulation{
						Species: "dog",
						Count:   1,
					},
					&SiteVisitPopulation{
						Species: "rat",
						Count:   5,
					},
				),
			},
			expectedVal: []byte("\x58" +
				"\x00\x00\x00\x24" +
				"\x00\x00\x30\x39" +
				"\x00\x00\x00\x02" +
				"\x00\x00\x00\x03" +
				"\x64\x6f\x67" +
				"\x00\x00\x00\x01" +
				"\x00\x00\x00\x03" +
				"\x72\x61\x74" +
				"\x00\x00\x00\x05" +
				"\x8c"),
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
