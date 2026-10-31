package logo

import "testing"

func TestBannersArePlainASCIIAndFit(t *testing.T) {
	for name, b := range map[string]string{"start": Start, "exit": Exit} {
		lines := Lines(b)
		if len(lines) < 3 {
			t.Errorf("%s: too few lines", name)
		}
		if w := Width(b); w == 0 || w > 60 {
			t.Errorf("%s: width %d", name, w)
		}
		for _, l := range lines {
			for _, r := range l {
				if r < ' ' || r > '~' {
					t.Errorf("%s: non-ASCII or control character %q", name, r)
				}
			}
		}
	}
}
