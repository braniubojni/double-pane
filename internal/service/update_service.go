package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/config"
	"github.com/erikharutyunyan/double-pane/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	githubOwner = "braniubojni"
	githubRepo  = "double-pane"
)

// UpdateService is a thin Wails-bound façade over app.Updater.
type UpdateService struct {
	app *application.App
}

func NewUpdateService() *UpdateService {
	return &UpdateService{}
}

// AttachUpdateApp injects the application after application.New (and Updater.Init).
// Package-level (not a service method) so it is not exposed in Wails frontend bindings.
func AttachUpdateApp(s *UpdateService, app *application.App) {
	s.app = app
}

// GetVersion returns the build-injected app version (no leading v).
func (s *UpdateService) GetVersion() string {
	return version.Version
}

// ReleasesURL is the human-facing releases page.
func (s *UpdateService) ReleasesURL() string {
	return fmt.Sprintf("https://github.com/%s/%s/releases", githubOwner, githubRepo)
}

// CheckAndInstall opens the Wails update window, checks GitHub Releases, and
// if a newer version is found, downloads/verifies/stages it for Restart & Apply.
// Errors propagate to the frontend so the caller can show failures.
func (s *UpdateService) CheckAndInstall() error {
	if s.app == nil {
		return fmt.Errorf("updater not ready")
	}
	return s.app.Updater.CheckAndInstall(context.Background())
}

// CheckForUpdate checks GitHub Releases without downloading or installing.
// Returns the newer version string, or "" if already up to date.
func (s *UpdateService) CheckForUpdate() (string, error) {
	if s.app == nil {
		return "", fmt.Errorf("updater not ready")
	}
	rel, err := s.app.Updater.Check(context.Background())
	if err != nil {
		return "", err
	}
	if rel == nil {
		return "", nil
	}
	return rel.Version, nil
}

// OpenReleasesPage opens the GitHub releases page in the browser.
func (s *UpdateService) OpenReleasesPage() error {
	return config.OpenInOS(s.ReleasesURL())
}

// CurrentVersionForUpdater returns a version string suitable for updater.Config
// (no leading "v"). Dev builds keep their string as-is.
func CurrentVersionForUpdater(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "0.0.0-dev"
	}
	return strings.TrimPrefix(v, "v")
}
