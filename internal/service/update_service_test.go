package service

import "testing"

func TestCurrentVersionForUpdater(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "0.0.0-dev"},
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"0.0.0-dev", "0.0.0-dev"},
		{"  v2.0.0  ", "2.0.0"},
	}
	for _, tt := range tests {
		if got := CurrentVersionForUpdater(tt.in); got != tt.want {
			t.Errorf("CurrentVersionForUpdater(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCheckForUpdate_NotReady(t *testing.T) {
	s := NewUpdateService()
	got, err := s.CheckForUpdate()
	if err == nil {
		t.Fatal("expected error when updater app is nil")
	}
	if got != "" {
		t.Errorf("got version %q, want empty", got)
	}
	if err.Error() != "updater not ready" {
		t.Errorf("error = %q, want %q", err.Error(), "updater not ready")
	}
}
