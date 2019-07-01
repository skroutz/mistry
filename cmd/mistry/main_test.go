package main

import (
	"reflect"
	"testing"

	"github.com/urfave/cli"
)

func TestParseDynamicArgs(t *testing.T) {
	cases := []struct {
		In       cli.Args
		Expected map[string]string
	}{
		{cli.Args{"--a", "b", "c=d", "e=f", "--g", "h"},
			map[string]string{"a": "b", "c": "d", "e": "f", "g": "h"}},

		{cli.Args{"a=b", "--@c", "d", "e=f", "--g", "h"},
			map[string]string{"a": "b", "@c": "d", "e": "f", "g": "h"}},

		{cli.Args{"a=b", "--c", "d"},
			map[string]string{"a": "b", "c": "d"}},

		{cli.Args{"--a", "b", "@c=d"},
			map[string]string{"a": "b", "@c": "d"}},

		{cli.Args{"a=b", "c=d"},
			map[string]string{"a": "b", "c": "d"}},

		{cli.Args{"a", "b", "--c", "d"},
			map[string]string{"a": "b", "c": "d"}},

		{cli.Args{"--a", "b"}, map[string]string{"a": "b"}},
		{cli.Args{"a=b"}, map[string]string{"a": "b"}},
	}

	for _, c := range cases {
		actual := parseDynamicArgs(c.In)

		if !reflect.DeepEqual(actual, c.Expected) {
			t.Errorf("expected %v, got %v", c.Expected, actual)
		}
	}
}
