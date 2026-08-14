package fleet

import (
	"runtime"
	"testing"

	"github.com/udit-001/dock/internal/catalog"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		state             InstalledState
		installed, latest string
		want              Status
	}{
		{StateNotInstalled, "", "v0.9.3", NotInstalled},
		{StateInstalled, "v0.9.3", "v0.9.3", UpToDate},
		{StateInstalled, "v0.9.2", "v0.9.3", UpgradeAvailable},
		{StateInstalled, "v1.0.0", "v0.9.3", UpToDate},
		{StateInstalled, "dev", "v0.9.3", UpgradeAvailable}, // old pseudo-version → update
		{StateVersionUnknown, "", "v0.9.3", Unknown},        // present, unknown version
	}
	for _, c := range cases {
		if got := Decide(c.state, c.installed, c.latest); got != c.want {
			t.Errorf("Decide(%v,%q,%q)=%v want %v", c.state, c.installed, c.latest, got, c.want)
		}
	}
}

func TestSelectAsset(t *testing.T) {
	key := PlatformKey()
	assets := map[string]catalog.Asset{
		key:          {URL: "correct"},
		"other/arch": {URL: "wrong"},
	}
	got, ok := SelectAsset(assets)
	if !ok || got.URL != "correct" {
		t.Fatalf("SelectAsset should return current platform asset, got %+v ok=%v", got, ok)
	}
}

func TestSelectAssetNone(t *testing.T) {
	if _, ok := SelectAsset(map[string]catalog.Asset{}); ok {
		t.Fatal("expected no asset for empty map")
	}
	_ = runtime.GOOS // keep import
}
