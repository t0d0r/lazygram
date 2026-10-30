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
		{"/chats dev", Command{Chats, "dev"}, false},
		{"/ls", Command{Chats, ""}, false},
		{"/join  #dev team ", Command{Select, "#dev team"}, false},
		{"/filter", Command{Filter, ""}, false},
		{"/filter foo bar", Command{Filter, "foo bar"}, false},
		{"/all", Command{All, ""}, false},
		{"/history", Command{History, ""}, false},
		{"/open dev", Command{History, "dev"}, false},
		{"/search needle", Command{Search, "needle"}, false},
		{"/select", Command{}, true},
		{"/search", Command{}, true},
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
