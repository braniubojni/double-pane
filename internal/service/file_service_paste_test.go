package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/clipboard"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

func TestPasteMode(t *testing.T) {
	t.Parallel()
	archive := filepath.Join(t.TempDir(), "docs.zip")
	if err := os.WriteFile(archive, []byte("pk"), 0o644); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(archive, "docs")

	cases := []struct {
		name    string
		dest    string
		files   []string
		png     []byte
		want    string
		wantErr error
		errSub  string
	}{
		{
			name:  "local files",
			dest:  t.TempDir(),
			files: []string{"/tmp/a.txt"},
			want:  pasteLocal,
		},
		{
			name:  "ssh files",
			dest:  "ssh://u@h:22/home",
			files: []string{"/tmp/a.txt", "/tmp/dir"},
			png:   []byte("ignored"),
			want:  pasteUpload,
		},
		{
			name:  "smb files",
			dest:  "smb://u@h:445/Share",
			files: []string{"/tmp/a.txt"},
			want:  pasteUpload,
		},
		{
			name:  "mega files",
			dest:  "mega://u@gmail.com/Cloud",
			files: []string{"/tmp/a.txt"},
			want:  pasteUpload,
		},
		{
			name: "ssh png",
			dest: "ssh://u@h:22/home",
			png:  []byte{0x89, 'P', 'N', 'G'},
			want: pasteUploadPNG,
		},
		{
			name:    "ssh empty",
			dest:    "ssh://u@h:22/home",
			wantErr: clipboard.ErrEmpty,
		},
		{
			name:    "trash",
			dest:    "trash://",
			files:   []string{"/tmp/a.txt"},
			wantErr: filesystem.ErrTrashReadOnly,
		},
		{
			name:    "archive",
			dest:    inner,
			files:   []string{"/tmp/a.txt"},
			wantErr: filesystem.ErrArchiveReadOnly,
		},
		{
			name:   "ssh source",
			dest:   "ssh://u@h:22/home",
			files:  []string{"/tmp/a.txt", "ssh://u@h:22/other"},
			errSub: "clipboard has a remote path",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := pasteMode(tc.dest, tc.files, tc.png)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if tc.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("mode %q", got)
			}
		})
	}
}

func TestPasteRemoteUploadsLocalFiles(t *testing.T) {
	t.Parallel()
	s := NewFileService(nil, nil, nil, t.TempDir())
	src := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotSrc, gotDest string
	s.pasteUpload = func(_ context.Context, sources []string, destDir string, _ filesystem.ProgressFunc) error {
		if len(sources) != 1 {
			t.Errorf("sources=%v", sources)
		} else {
			gotSrc = sources[0]
		}
		gotDest = destDir
		return nil
	}
	dest := "ssh://u@h:22/home"
	if err := s.pasteRemote(dest, []string{src}, []byte("png-ignored"), time.Now()); err != nil {
		t.Fatal(err)
	}
	if gotSrc != src || gotDest != dest {
		t.Fatalf("upload src=%q dest=%q", gotSrc, gotDest)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal(err)
	}
}

func TestPasteRemoteUsesUploadNotCopy(t *testing.T) {
	t.Parallel()
	s := NewFileService(nil, nil, nil, t.TempDir())
	src := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := s.pasteRemote("smb://u@h:445/Share", []string{src}, nil, time.Now())
	if err == nil || !strings.Contains(err.Error(), "remote not available") {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Fatal(statErr)
	}
}

func TestPasteRemotePNGCleanup(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 5, 16, 29, 0, 0, time.UTC)
	wantBase := "clipboard-20261005162900.png"
	png := []byte{0x89, 'P', 'N', 'G'}

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		s := NewFileService(nil, nil, nil, t.TempDir())
		var temp string
		s.pasteUpload = func(_ context.Context, sources []string, _ string, _ filesystem.ProgressFunc) error {
			if len(sources) != 1 {
				t.Errorf("sources=%v", sources)
				return errors.New("upload failed")
			}
			temp = sources[0]
			if filepath.Base(temp) != wantBase {
				t.Errorf("name %q", filepath.Base(temp))
			}
			b, err := os.ReadFile(temp)
			if err != nil {
				t.Errorf("read temp: %v", err)
			} else if !bytes.Equal(b, png) {
				t.Errorf("bytes %x", b)
			}
			return errors.New("upload failed")
		}
		err := s.pasteRemote("mega://u@gmail.com/Cloud", nil, png, now)
		if err == nil || err.Error() != "upload failed" {
			t.Fatalf("got %v", err)
		}
		if temp == "" {
			t.Fatal("upload not called")
		}
		if _, statErr := os.Stat(temp); !os.IsNotExist(statErr) {
			t.Fatalf("temp remains: %v", statErr)
		}
		if _, statErr := os.Stat(filepath.Dir(temp)); !os.IsNotExist(statErr) {
			t.Fatalf("temp dir remains: %v", statErr)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		t.Parallel()
		s := NewFileService(nil, nil, nil, t.TempDir())
		entered := make(chan struct{})
		var temp string
		s.pasteUpload = func(ctx context.Context, sources []string, _ string, _ filesystem.ProgressFunc) error {
			temp = sources[0]
			if _, err := os.Stat(temp); err != nil {
				t.Errorf("temp missing during upload: %v", err)
			}
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		}
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.pasteRemote("ssh://u@h:22/home", nil, png, now)
		}()
		<-entered
		var id string
		s.jobs.Range(func(k, _ any) bool {
			id, _ = k.(string)
			return false
		})
		if id == "" {
			t.Fatal("job not stored")
		}
		if err := s.CancelJob(id); err != nil {
			t.Fatal(err)
		}
		if err := <-errCh; !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
		if _, statErr := os.Stat(temp); !os.IsNotExist(statErr) {
			t.Fatalf("temp remains: %v", statErr)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		s := NewFileService(nil, nil, nil, t.TempDir())
		var temp string
		s.pasteUpload = func(_ context.Context, sources []string, _ string, _ filesystem.ProgressFunc) error {
			temp = sources[0]
			return nil
		}
		if err := s.pasteRemote("ssh://u@h:22/home", nil, png, now); err != nil {
			t.Fatal(err)
		}
		if _, statErr := os.Stat(temp); !os.IsNotExist(statErr) {
			t.Fatalf("temp remains: %v", statErr)
		}
	})
}
