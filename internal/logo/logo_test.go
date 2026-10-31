package logo

import (
	"strings"
	"testing"
)

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

func TestExitFacesArePlainASCIIAndDistinct(t *testing.T) {
	seen := map[string]bool{}
	for i, e := range exits {
		if seen[e] {
			t.Errorf("face %d repeats another", i)
		}
		seen[e] = true
		if w := Width(e); w == 0 || w > 80 {
			t.Errorf("face %d: width %d", i, w)
		}
		if len(Lines(e)) < 2 {
			t.Errorf("face %d: too few lines", i)
		}
		for _, l := range Lines(e) {
			for _, r := range l {
				if r < ' ' || r > '~' {
					t.Errorf("face %d: non-ASCII %q", i, r)
				}
			}
		}
	}
	if len(Variants(120)) != len(exits) || len(Variants(10)) != 1 || Variants(10)[0] != Narrow {
		t.Errorf("Variants filters by width: %d %d", len(Variants(120)), len(Variants(10)))
	}
	picked := map[string]bool{}
	for i := 0; i < 200; i++ {
		picked[Random(120)] = true
	}
	if len(picked) < 2 {
		t.Error("Random should vary")
	}
}

func TestPixelFontDrawsEveryLetter(t *testing.T) {
	for r, g := range pixelFont {
		if len(g) != pixelRows {
			t.Errorf("%c: %d rows", r, len(g))
		}
		for _, row := range g {
			if len(row) != len(g[0]) {
				t.Errorf("%c: ragged row %q", r, row)
			}
		}
	}
	plain := pixels("lazygram", "##", "")
	if w := Width(plain); w != 6+6*8+10+7 || len(Lines(plain)) != pixelRows || !strings.Contains(plain, "##") {
		t.Fatalf("plain %d cols:\n%s", w, plain)
	}
	shadowed := pixels("lazygram", "##", "::")
	if len(Lines(shadowed)) != pixelRows+1 || !strings.HasPrefix(Lines(shadowed)[pixelRows], "::::::") || Width(shadowed) != Width(plain) {
		t.Fatalf("shadow:\n%s", shadowed)
	}
	if pixels("xy", "##", "") != pixels("y", "##", "") {
		t.Fatal("unknown letters are skipped")
	}
}
