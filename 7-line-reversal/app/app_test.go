package app

import (
	"bytes"
	"io"
	"testing"
)

func TestApp_Handle(t *testing.T) {
	a := &App{}
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name  string
		args  args
		wantW string
	}{
		{
			name: "success",
			args: args{
				bytes.NewReader([]byte("abcde12345\n")),
			},
			wantW: "54321edcba\n",
		},
		{
			name: "success",
			args: args{
				bytes.NewReader([]byte("abcde1234\n")),
			},
			wantW: "4321edcba\n",
		},
		{
			name: "success",
			args: args{
				bytes.NewReader([]byte("abcde12345")),
			},
			wantW: "54321edcba",
		},
		{
			name: "success",
			args: args{
				bytes.NewReader([]byte("abcde1234")),
			},
			wantW: "4321edcba",
		},
		{
			name: "success",
			args: args{
				bytes.NewReader([]byte("")),
			},
			wantW: "",
		},
		{
			name: "success",
			args: args{
				bytes.NewReader([]byte("\n")),
			},
			wantW: "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			a.Handle(w, tt.args.r)
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("Handle() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}
