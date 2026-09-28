package cli

import "testing"

func TestParseScreenFlags(t *testing.T) {
	cases := []struct {
		in    []string
		kind  string
		steal bool
	}{
		{nil, "up", false},
		{[]string{"up"}, "up", false},
		{[]string{"-r"}, "attach", false},
		{[]string{"attach"}, "attach", false},
		{[]string{"r", "abc", "--go"}, "attach", false},
		{[]string{"-d"}, "detach", false},
		{[]string{"-d", "-r"}, "attach", true},
		{[]string{"-r", "-d"}, "attach", true},
		{[]string{"-ls"}, "list", false},
		{[]string{"ls"}, "list", false},
		{[]string{"daemon"}, "daemon", false},
		{[]string{"down"}, "down", false},
		{[]string{"up", "-d"}, "up", true},
		{[]string{"send", "--from", "nova"}, "send", false},
		{[]string{"listen", "--as", "nova"}, "listen", false},
	}
	for _, c := range cases {
		a, err := Parse(c.in)
		if err != nil {
			t.Fatalf("%v: %v", c.in, err)
		}
		if a.Kind != c.kind || a.Steal != c.steal {
			t.Fatalf("%v: got kind=%s steal=%v want %s %v", c.in, a.Kind, a.Steal, c.kind, c.steal)
		}
	}
}
