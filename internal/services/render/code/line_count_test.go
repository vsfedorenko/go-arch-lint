package code

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_lineCounter(t *testing.T) {
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "simple",
			args: args{
				r: bytes.NewReader([]byte("Hello world\nThis buffer has three lines\n")),
			},
			want:    3,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lineCounter(tt.args.r)
			assert.Equal(t, tt.wantErr, err != nil, "lineCounter() error = %v, wantErr ")
			assert.Equal(t, tt.want, got, "lineCounter() got")
		})
	}
}
