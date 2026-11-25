package hashtag

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		raw, name, display string
		ok                 bool
	}{
		{"Fruit", "fruit", "Fruit", true},
		{"  #Fruit　", "fruit", "Fruit", true},
		{"##有機", "有機", "有機", true},
		{"", "", "", false},
		{"#", "", "", false},
		{" \t ", "", "", false},
	}
	for _, c := range cases {
		name, display, ok := Normalize(c.raw)
		if ok != c.ok || name != c.name || display != c.display {
			t.Errorf("Normalize(%q) = %q, %q, %v", c.raw, name, display, ok)
		}
	}
}

func TestRequireSetKinds(t *testing.T) {
	got, err := RequireSet([]string{"#Fruit", "FRUIT", "水果", "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tag := range got {
		names = append(names, tag.Name)
	}
	if !slices.Equal(names, []string{"fruit", "水果", "abc123"}) || got[0].Display != "Fruit" {
		t.Fatalf("%+v", got)
	}
	if got, err := RequireSet(nil); got != nil || err != nil {
		t.Fatalf("empty input: %v %v", got, err)
	}
	cases := []struct {
		in   []string
		want error
	}{
		{[]string{""}, ErrEmpty},
		{[]string{"#"}, ErrEmpty},
		{[]string{"a b"}, ErrInvalidChars},
		{[]string{"a-b"}, ErrInvalidChars},
		{[]string{"é"}, ErrInvalidChars},
		{[]string{strings.Repeat("a", MaxLength+1)}, ErrLength},
	}
	for _, c := range cases {
		if _, err := RequireSet(c.in); !errors.Is(err, c.want) {
			t.Errorf("%q: got %v, want %v", c.in, err, c.want)
		}
	}
	if _, err := RequireSet([]string{strings.Repeat("a", MaxLength)}); err != nil {
		t.Errorf("max length allowed: %v", err)
	}
	var many []string
	for i := range MaxCount + 1 {
		many = append(many, strings.Repeat("a", i+1))
	}
	if _, err := RequireSet(many); !errors.Is(err, ErrTooMany) {
		t.Errorf("over max: %v", err)
	}
	if _, err := RequireSet(many[:MaxCount]); err != nil {
		t.Errorf("exactly max: %v", err)
	}
	if got, err := RequireSet([]string{"a", "A", "a"}); err != nil || len(got) != 1 {
		t.Errorf("duplicates do not count: %v %v", got, err)
	}
}
