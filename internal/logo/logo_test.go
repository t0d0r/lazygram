package logo

import "testing"

func TestBannersArePlainASCIIAndFit(t *testing.T) {
	for name, b := range map[string]string{"wide": Wide, "narrow": Narrow} {
		lines := Lines(b)
		if len(lines) < 3 {
			t.Errorf("%s: too few lines", name)
		}
		if w := Width(b); w == 0 || w > 80 {
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
	if Width(Narrow) >= Width(Wide) || Width(Narrow) > 50 {
		t.Errorf("narrow %d, wide %d", Width(Narrow), Width(Wide))
	}
}

func TestPick(t *testing.T) {
	if Pick(120) != Wide || Pick(Width(Wide)) != Wide || Pick(Width(Wide)-1) != Narrow || Pick(20) != Narrow {
		t.Fatal("Pick chooses by width")
	}
}
