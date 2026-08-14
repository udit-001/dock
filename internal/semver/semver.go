// Package semver provides a minimal semver parse + compare sufficient for
// fleet release versions like "v0.9.3". Non-numeric suffixes are treated as
// pre-release (older than the same core triple).
package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// versionRe lexes the first semver-looking triple from arbitrary CLI output
// (e.g. "pharos version v0.9.3" → "v0.9.3").
var versionRe = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)

// Extract lexes a semver triple from arbitrary CLI output, reporting whether
// one was found. The returned string is normalized to the "vX.Y.Z" form that
// Parse accepts (e.g. "pharos version v0.9.3" → "v0.9.3", true).
func Extract(s string) (string, bool) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return "v" + m[1] + "." + m[2] + "." + m[3], true
}

// Version is a parsed semver triple plus an optional pre-release suffix.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
	dev                 bool
}

// Parse parses a version string, tolerating a leading "v". Pseudo-versions
// like "0.8.2-0.2026..." are collapsed to "0.8.2-pre"; "dev"/"" → 0.0.0-dev.
func Parse(s string) (Version, error) {
	v := Version{dev: true}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" || s == "dev" || s == "(devel)" {
		return Version{Pre: "dev", dev: true}, nil
	}
	core := s
	if i := strings.IndexByte(s, '-'); i >= 0 {
		core = s[:i]
		v.Pre = strings.TrimPrefix(s[i+1:], "0.")
	}
	parts := strings.Split(core, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return v, fmt.Errorf("semver %q: bad core", s)
	}
	nums := []*int{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, fmt.Errorf("semver %q: %w", s, err)
		}
		*nums[i] = n
	}
	for _, n := range nums[len(parts):] {
		*n = 0
	}
	if v.Pre == "" {
		v.dev = false
	}
	return v, nil
}

// Compare returns -1, 0, or +1. Pre-releases sort below the same core.
func (a Version) Compare(b Version) int {
	if c := cmpInt(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmpInt(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmpInt(a.Patch, b.Patch); c != 0 {
		return c
	}
	return cmpPre(a.Pre, b.Pre)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// cmpPre orders pre-release suffixes: a release (empty) is newer than any
// pre-release; otherwise compare lexically, stable-build dirties last.
func cmpPre(a, b string) int {
	switch {
	case a == "" && b != "":
		return 1
	case a != "" && b == "":
		return -1
	case a == "" && b == "":
		return 0
	}
	return strings.Compare(a, b)
}
