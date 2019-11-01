package flags

import (
	"reflect"
	"testing"

	"github.com/urfave/cli"
)

func TestConcat(t *testing.T) {
	type args struct {
		first []cli.Flag
		rest  [][]cli.Flag
	}
	tests := []struct {
		name string
		args args
		want []cli.Flag
	}{{
		name: "Concat one list",
		args: args{
			first: []cli.Flag{cli.StringFlag{
				Name: "foo",
			},
			},
			rest: nil,
		},
		want: []cli.Flag{cli.StringFlag{Name: "foo"}},
	}, {
		name: "Concat two lists",
		args: args{
			first: []cli.Flag{cli.StringFlag{
				Name: "foo",
			},
			},
			rest: [][]cli.Flag{
				[]cli.Flag{
					cli.StringFlag{Name: "bar"},
				},
			},
		},
		want: []cli.Flag{cli.StringFlag{Name: "foo"}, cli.StringFlag{Name: "bar"}},
	}, {
		name: "Concat three lists",
		args: args{
			first: []cli.Flag{cli.StringFlag{
				Name: "foo",
			},
			},
			rest: [][]cli.Flag{
				[]cli.Flag{
					cli.StringFlag{Name: "bar"},
				},
				[]cli.Flag{
					cli.BoolFlag{Name: "baz"},
				},
			},
		},
		want: []cli.Flag{cli.StringFlag{Name: "foo"}, cli.StringFlag{Name: "bar"}, cli.BoolFlag{Name: "baz"}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Concat(tt.args.first, tt.args.rest...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Concat() = %v, want %v", got, tt.want)
			}
		})
	}
}
