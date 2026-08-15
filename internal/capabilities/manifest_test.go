package capabilities

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestMatchesCommittedFile(t *testing.T) {
	want, err := RenderManifest()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repoRoot(t), filepath.FromSlash(ManifestRelPath))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run make generate)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run make generate\n--- committed ---\n%s\n--- render ---\n%s", ManifestRelPath, got, want)
	}
}

func TestManifestRoundTripIDs(t *testing.T) {
	raw, err := RenderManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(ManifestAPIVersion)) {
		t.Fatal("missing apiVersion")
	}
	if !bytes.Contains(raw, []byte(ManifestGeneratedBy)) {
		t.Fatal("missing generatedBy")
	}
	for _, c := range All() {
		if !bytes.Contains(raw, []byte(`"id": "`+string(c.ID)+`"`)) {
			t.Errorf("manifest missing id %s", c.ID)
		}
	}
}
