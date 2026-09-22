package service

import (
	"io"

	"github.com/erikharutyunyan/double-pane/internal/remote"
)

// remoteShell is an interactive SSH shell, wrapping a remote.ShellSession.
type remoteShell struct {
	sess   *remote.ShellSession
	reader io.Reader
}

// spawnRemotePTY opens an interactive SSH shell cd'd into vpath's remote directory.
func spawnRemotePTY(mgr *remote.Manager, vpath string, cols, rows int) (ptyHandle, error) {
	sess, reader, err := mgr.OpenShell(vpath, cols, rows)
	if err != nil {
		return nil, err
	}
	return &remoteShell{sess: sess, reader: reader}, nil
}

func (r *remoteShell) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *remoteShell) Write(data string) error {
	return r.sess.Write(data)
}

func (r *remoteShell) Resize(cols, rows int) error {
	return r.sess.Resize(cols, rows)
}

func (r *remoteShell) Close() error {
	return r.sess.Close()
}

// ExitCode is not tracked for remote shells (best-effort).
// Wire up ssh.Session.Wait() if a real exit code is ever needed.
func (r *remoteShell) ExitCode() int {
	return 0
}
