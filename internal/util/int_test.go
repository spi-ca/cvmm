package util

import (
	"reflect"
	"testing"
)

func TestConsecutiveRanges(t *testing.T) {
	type args struct {
		input []int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test 1",
			args: args{
				input: []int{1, 2, 3, 6, 7},
			},
			want: "1-3,6-7",
		},
		{
			name: "test 2",
			args: args{
				input: []int{-1, 0, 1, 2, 5, 6, 8},
			},
			want: "-1-2,5-6,8",
		},
		{
			name: "test 3",
			args: args{
				input: []int{-1, 3, 4, 5, 20, 21, 25},
			},
			want: "-1,3-5,20-21,25",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConsecutiveRanges(tt.args.input).String(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConsecutiveRanges() = %v, want %v", got, tt.want)
			}
		})
	}
}
