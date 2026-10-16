package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func TestTreeUsageRefusesWithoutDone(t *testing.T) {
	emitted := false
	s := &FileService{onEvent: func(name string, _ any) {
		if name == "usage:done" {
			emitted = true
		}
	}}
	root := t.TempDir()
	zipPath := filepath.Join(root, "bundle.zip")
	if err := os.WriteFile(zipPath, []byte("pk"), 0o644); err != nil {
		t.Fatal(err)
	}
	vaultRoot := filepath.Join(root, "vault")
	if err := os.MkdirAll(filepath.Join(vaultRoot, ".dpvault"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vaultRoot, ".dpvault", "manifest.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		job  string
		path string
	}{
		{job: "", path: root},
		{job: "job", path: "trash://"},
		{job: "job", path: zipPath},
		{job: "job", path: "ssh://u@h/p"},
		{job: "job", path: vaultRoot},
	}
	for _, c := range cases {
		emitted = false
		err := s.TreeUsage(c.job, c.path, 0)
		if err == nil {
			t.Fatalf("TreeUsage(%q, %q) want error", c.job, c.path)
		}
		if emitted {
			t.Fatalf("TreeUsage(%q, %q) emitted usage:done", c.job, c.path)
		}
	}
}

func TestFitUsagePayloadStripsDeepestKids(t *testing.T) {
	done := &domain.UsageDone{
		Denied: []string{},
		Total:  10,
		Tree: &domain.UsageNode{
			Name: "r", IsDir: true, Size: 10, Path: "/r",
			Kids: []domain.UsageNode{{
				Name: "a", IsDir: true, Size: 10, Path: "/r/a",
				Kids: []domain.UsageNode{{Name: "f", Path: "/r/a/f", Size: 10}},
			}},
		},
	}
	fitUsagePayload(done, 1)
	if !done.Truncated {
		t.Fatal("expected truncated")
	}
	if done.Tree.Size != 10 || len(done.Tree.Kids) != 0 {
		t.Fatalf("root should stay a sized leaf, got %#v", done.Tree)
	}
}
