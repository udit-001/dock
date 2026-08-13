package registry

import "testing"

func TestResolveAssetWindowsPrefersExe(t *testing.T) {
	rel := &ghRelease{TagName: "v0.9.3", Assets: []ghAsset{
		{Name: "pharos_0.9.3_windows_amd64.zip", BrowserDownloadURL: "zip"},
		{Name: "pharos_0.9.3_windows_amd64.exe", BrowserDownloadURL: "exe"},
	}}
	got, err := resolveAsset(rel, "pharos", "v0.9.3", "windows/amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got.BrowserDownloadURL != "exe" {
		t.Errorf("windows should prefer .exe, got %q", got.BrowserDownloadURL)
	}
}

func TestResolveAssetWindowsZipOnly(t *testing.T) {
	rel := &ghRelease{TagName: "v0.12.0", Assets: []ghAsset{
		{Name: "waypoint_0.12.0_windows_amd64.zip", BrowserDownloadURL: "zip"},
	}}
	got, err := resolveAsset(rel, "waypoint", "v0.12.0", "windows/amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got.BrowserDownloadURL != "zip" {
		t.Errorf("fall back to .zip when no .exe, got %q", got.BrowserDownloadURL)
	}
}

func TestResolveAssetLinuxTarGz(t *testing.T) {
	rel := &ghRelease{TagName: "v0.2.0", Assets: []ghAsset{
		{Name: "harbor_0.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "tgz"},
	}}
	got, err := resolveAsset(rel, "harbor", "v0.2.0", "linux/amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got.BrowserDownloadURL != "tgz" {
		t.Errorf("linux should use .tar.gz, got %q", got.BrowserDownloadURL)
	}
}

func TestResolveAssetMissingPlatform(t *testing.T) {
	rel := &ghRelease{TagName: "v0.1.0", Assets: []ghAsset{
		{Name: "app_0.1.0_linux_amd64.tar.gz"},
	}}
	if _, err := resolveAsset(rel, "app", "v0.1.0", "darwin/arm64"); err == nil {
		t.Error("expected error for missing platform asset")
	}
}