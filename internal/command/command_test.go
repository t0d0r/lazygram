package command

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Command
		err  bool
	}{
		{"/help", Command{Help, ""}, false},
		{"  /Q ", Command{Quit, ""}, false},
		{"/sync", Command{Refresh, ""}, false},
		{"/list dev", Command{List, "dev"}, false},
		{"/ls", Command{List, ""}, false},
		{"/win", Command{Win, ""}, false},
		{"/win 2", Command{Win, "2"}, false},
		{"/w  @linus ", Command{Win, "@linus"}, false},
		{"/join #dev", Command{Win, "#dev"}, false},
		{"/close", Command{Close, ""}, false},
		{"/close 3", Command{Close, "3"}, false},
		{"/last", Command{Last, ""}, false},
		{"/history 20", Command{Last, "20"}, false},
		{"/search", Command{Search, ""}, false},
		{"/search needle", Command{Search, "needle"}, false},
		{"/stats", Command{Stats, ""}, false},
		{"/whois @ada", Command{Whois, "@ada"}, false},
		{"/me waves", Command{Me, "waves"}, false},
		{"/press 2", Command{Press, "2"}, false},
		{"/btn 1234 1", Command{Press, "1234 1"}, false},
		{"/open", Command{Open, ""}, false},
		{"/download 42", Command{Open, "42"}, false},
		{"/theme", Command{Theme, ""}, false},
		{"/colors nord", Command{Theme, "nord"}, false},
		{"/move 3", Command{Move, "3"}, false},
		{"/bogus", Command{}, true},
		{"/", Command{}, true},
		{"hello", Command{}, true},
		{"", Command{}, true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if (err != nil) != c.err {
			t.Errorf("%q: err=%v want err=%v", c.in, err, c.err)
			continue
		}
		if !c.err && got != c.want {
			t.Errorf("%q: got %+v want %+v", c.in, got, c.want)
		}
	}
}

func TestNamesSortedWithSlash(t *testing.T) {
	n := Names()
	if len(n) != len(table) || n[0][0] != '/' {
		t.Fatal(n)
	}
	for i := 1; i < len(n); i++ {
		if n[i-1] >= n[i] {
			t.Fatal("not sorted")
		}
	}
}
