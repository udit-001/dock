package semver

import "testing"

func TestExtract(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"pharos version v0.9.3", "v0.9.3", true},
		{"v0.9.3", "v0.9.3", true},
		{"waypoint version v0.12.0", "v0.12.0", true},
		{"dev build", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := Extract(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("Extract(%q)=%q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.9.3", "v0.9.3", 0},
		{"v0.9.3", "v0.2.0", 1},
		{"v0.2.0", "v0.9.3", -1},
		{"v0.12.0", "v0.2.9", 1}, // 12 > 2 numerically, not lexically
		{"v0.9.3", "v0.9.2", 1},
		{"v1.0.0", "v0.9.9", 1},
		{"v0.9.3", "", 1}, // "" parses as dev; a real version is newer
		{"", "v0.1.0", -1},
	}
	for _, c := range cases {
		av, errA := Parse(c.a)
		bv, errB := Parse(c.b)
		if errA != nil {
			t.Fatalf("Parse(%q): %v", c.a, errA)
		}
		if errB != nil {
			t.Fatalf("Parse(%q): %v", c.b, errB)
		}
		if got := av.Compare(bv); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParsePre(t *testing.T) {
	v, err := Parse("0.9.3-0.20260806")
	if err != nil {
		t.Fatal(err)
	}
	rel, _ := Parse("0.9.3")
	if rel.Compare(v) <= 0 {
		t.Errorf("release 0.9.3 should be newer than pre 0.9.3-pre")
	}
}

func TestParseDev(t *testing.T) {
	if _, err := Parse("dev"); err != nil {
		t.Fatalf("dev should parse: %v", err)
	}
	if _, err := Parse(""); err != nil {
		t.Fatalf("empty should parse: %v", err)
	}
}
