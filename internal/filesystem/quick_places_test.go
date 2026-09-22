package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

func withUserDirs(t *testing.T, dirs xdg.UserDirectories) {
	t.Helper()
	orig := xdg.UserDirs
	xdg.UserDirs = dirs
	t.Cleanup(func() { xdg.UserDirs = orig })
}

func TestQuickPlacesAllPresent(t *testing.T) {
	root := t.TempDir()
	names := []string{"Desktop", "Documents", "Download", "Pictures", "Music", "Videos"}
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	withUserDirs(t, xdg.UserDirectories{
		Desktop:   filepath.Join(root, "Desktop"),
		Documents: filepath.Join(root, "Documents"),
		Download:  filepath.Join(root, "Download"),
		Pictures:  filepath.Join(root, "Pictures"),
		Music:     filepath.Join(root, "Music"),
		Videos:    filepath.Join(root, "Videos"),
	})

	places := QuickPlaces()

	wantOrder := []string{"Home", "Desktop", "Documents", "Downloads", "Pictures", "Music", "Videos"}
	if len(places) != len(wantOrder) {
		t.Fatalf("got %d places, want %d: %+v", len(places), len(wantOrder), places)
	}
	for i, name := range wantOrder {
		if places[i].Name != name {
			t.Errorf("place[%d].Name = %q, want %q", i, places[i].Name, name)
		}
	}
}

func TestQuickPlacesOmitsMissing(t *testing.T) {
	root := t.TempDir()
	// Only Documents exists; the rest point at nonexistent paths.
	if err := os.MkdirAll(filepath.Join(root, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	withUserDirs(t, xdg.UserDirectories{
		Desktop:   filepath.Join(root, "no-such-desktop"),
		Documents: filepath.Join(root, "Documents"),
		Download:  filepath.Join(root, "no-such-downloads"),
		Pictures:  filepath.Join(root, "no-such-pictures"),
		Music:     filepath.Join(root, "no-such-music"),
		Videos:    filepath.Join(root, "no-such-videos"),
	})

	places := QuickPlaces()

	names := make([]string, len(places))
	for i, p := range places {
		names[i] = p.Name
	}
	if len(places) != 2 {
		t.Fatalf("got %v, want [Home Documents]", names)
	}
	if places[0].Name != "Home" || places[1].Name != "Documents" {
		t.Errorf("got %v, want [Home Documents]", names)
	}
}

func TestQuickPlacesHomeDirFailure(t *testing.T) {
	origHome, hadHome := os.LookupEnv("HOME")
	if err := os.Unsetenv("HOME"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadHome {
			_ = os.Setenv("HOME", origHome)
		}
	})

	if places := QuickPlaces(); places != nil {
		t.Errorf("QuickPlaces() = %+v, want nil when HomeDir() fails", places)
	}
}
