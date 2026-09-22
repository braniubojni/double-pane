package volumes

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// Manager lists OS mounts, attaches disk images, and watches /Volumes-style dirs.
type Manager struct {
	mu      sync.Mutex
	attach  map[string]string // mount point → source .dmg path
	stop    chan struct{}
	started bool
}

func NewManager() *Manager {
	return &Manager{attach: make(map[string]string)}
}

func (m *Manager) List() ([]domain.Volume, error) {
	vols, err := listOS()
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range vols {
		if src, ok := m.attach[vols[i].Path]; ok {
			vols[i].SourcePath = src
			vols[i].Kind = "disk-image"
		}
	}
	return vols, nil
}

func (m *Manager) Unmount(path string) error {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	if err := unmountOS(abs); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.attach, abs)
	m.mu.Unlock()
	return nil
}

func (m *Manager) AttachDiskImage(ctx context.Context, path, password string, onProgress func(float64)) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	if mp := m.mountForImage(abs); mp != "" {
		return mp, nil
	}
	mp, err := attachDiskImage(ctx, abs, password, onProgress)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.attach[mp] = abs
	m.mu.Unlock()
	return mp, nil
}

// IsEncryptedDiskImage reports whether path is an encrypted disk image.
func (m *Manager) IsEncryptedDiskImage(path string) (bool, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(abs); err != nil {
		return false, err
	}
	return imageIsEncrypted(abs), nil
}

func (m *Manager) ParentOverride(dir string) string {
	abs, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return ""
	}
	m.mu.Lock()
	dmg, ok := m.attach[abs]
	m.mu.Unlock()
	if !ok || dmg == "" {
		return ""
	}
	return filepath.Dir(dmg)
}

func (m *Manager) mountForImage(dmg string) string {
	m.mu.Lock()
	for mp, src := range m.attach {
		if src == dmg {
			m.mu.Unlock()
			if _, err := os.Stat(mp); err == nil {
				return mp
			}
			return ""
		}
	}
	m.mu.Unlock()
	return existingMountForImage(dmg)
}

// StartWatch polls mount points and calls onChange when the set changes.
func (m *Manager) StartWatch(onChange func()) {
	if onChange == nil {
		return
	}
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.stop = make(chan struct{})
	m.mu.Unlock()
	go m.loop(onChange)
}

func (m *Manager) loop(onChange func()) {
	prev := volumeSignature()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-tick.C:
			next := volumeSignature()
			if next != prev {
				prev = next
				onChange()
			}
		}
	}
}

func volumeSignature() string {
	vols, err := listOS()
	if err != nil {
		return ""
	}
	var b string
	for _, v := range vols {
		b += v.Path + "\n"
	}
	return b
}

func classify(path, fs string, images map[string]string, rootDevs map[string]struct{}) domain.Volume {
	name := filepath.Base(path)
	v := domain.Volume{Path: path, Name: name, Kind: "external", Unmountable: true}
	if _, boot := rootDevs[path]; boot {
		v.Kind = "internal"
		v.Unmountable = false
	}
	if isNetworkFS(fs) {
		v.Kind = "network"
		v.Unmountable = true
	}
	if src, ok := images[path]; ok {
		v.Kind = "disk-image"
		v.SourcePath = src
		v.Unmountable = true
	}
	return v
}
