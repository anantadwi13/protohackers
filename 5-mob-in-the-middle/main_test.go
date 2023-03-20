package main

import (
	"reflect"
	"testing"
)

func Test_bogusCoinModifier(t *testing.T) {
	modifier := bogusCoinModifier("testboguscoin")

	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "replaced",
			input: []byte("7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX Hi alice, please send payment to 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX"),
			want:  []byte("testboguscoin Hi alice, please send payment to testboguscoin testboguscoin"),
		},
		{
			name:  "replaced",
			input: []byte("Hi alice, please send payment to 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX"),
			want:  []byte("Hi alice, please send payment to testboguscoin"),
		},
		{
			name:  "replaced",
			input: []byte("7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX Hi alice, please send payment to"),
			want:  []byte("testboguscoin Hi alice, please send payment to"),
		},
		{
			name:  "replaced",
			input: []byte("Hi alice, please 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX send payment to"),
			want:  []byte("Hi alice, please testboguscoin send payment to"),
		},
		{
			name:  "replaced",
			input: []byte("Hi alice, please 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHXsend"),
			want:  []byte("Hi alice, please testboguscoin"),
		},
		{
			name:  "replaced",
			input: []byte("7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX"),
			want:  []byte("testboguscoin"),
		},
		{
			name:  "invalid",
			input: []byte("Hi alice, please 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHXsendpaymentto"),
			want:  []byte("Hi alice, please 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHXsendpaymentto"),
		},
		{
			name:  "invalid",
			input: []byte("Hi alice, please 1iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX"),
			want:  []byte("Hi alice, please 1iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX"),
		},
		{
			name:  "invalid",
			input: []byte("7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX@"),
			want:  []byte("7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX@"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modifier(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("bogusCoinModifier() = %q, want %q", got, tt.want)
			}
		})
	}
}
