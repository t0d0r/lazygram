package telegram

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Dev Talk":               "dev-talk",
		"  Ops!!Team  ":          "ops-team",
		"Семейный чат":           "семейный-чат",
		"A very long chat title": "a-very-long-ch",
		"---":                    "",
		"C++ & Go":               "c-go",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAliasForPrefersUsernameAndDedupes(t *testing.T) {
	taken := map[string]bool{}
	if a := aliasFor(entity{username: "SomeGroup", title: "Other"}, taken); a != "somegroup" {
		t.Fatal(a)
	}
	if a := aliasFor(entity{title: "Dev Talk"}, taken); a != "dev-talk" {
		t.Fatal(a)
	}
	if a := aliasFor(entity{title: "dev talk"}, taken); a != "dev-talk-2" {
		t.Fatal(a)
	}
	if a := aliasFor(entity{title: "Dev-Talk"}, taken); a != "dev-talk-3" {
		t.Fatal(a)
	}
	if a := aliasFor(entity{title: "???"}, taken); a != "chat" {
		t.Fatal(a)
	}
	long := aliasFor(entity{title: "A very long chat title"}, taken)
	long2 := aliasFor(entity{title: "A very long chat title"}, taken)
	if long != "a-very-long-ch" || long2 != "a-very-long-2" || len([]rune(long2)) > maxAlias {
		t.Fatalf("%q %q", long, long2)
	}
}
