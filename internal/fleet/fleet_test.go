package fleet

import (
	"runtime"
	"testing"

	"github.com/udit-001/app-store/internal/registry"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		installed, latest string
		want              Status
	}{
		{"", "v0.9.3", NotInstalled},
		{"v0.9.3", "v0.9.3", UpToDate},
		{"v0.9.2", "v0.9.3", UpgradeAvailable},
		{"v1.0.0", "v0.9.3", UpToDate},
		{"dev", "v0.9.3", UpgradeAvailable}, // old pseudo-version → update
	}
	for _, c := range cases {
		if got := Decide(c.installed, c.latest); got != c.want {
			t.Errorf("Decide(%q,%q)=%v want %v", c.installed, c.latest, got, c.want)
		}
	}
}

func TestSelectAsset(t *testing.T) {
	key := PlatformKey()
	assets := map[string]registry.Asset{
		key:          {URL: "correct"},
		"other/arch": {URL: "wrong"},
	}
	got, ok := SelectAsset(assets)
	if !ok || got.URL != "correct" {
		t.Fatalf("SelectAsset should return current platform asset, got %+v ok=%v", got, ok)
	}
}

func TestSelectAssetNone(t *testing.T) {
	if _, ok := SelectAsset(map[string]registry.Asset{}); ok {
		t.Fatal("expected no asset for empty map")
	}
	_ = runtime.GOOS // keep import
}