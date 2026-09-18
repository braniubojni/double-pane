package filesystem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// HashFile SHA-256s path by streaming; it does not buffer the whole file.
func HashFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	return HashReader(ctx, f)
}

// HashReader SHA-256s r with io.Copy. If r is a Closer it is closed, including
// on ctx cancel so a blocked remote read can unblock.
func HashReader(ctx context.Context, r io.Reader) (string, error) {
	if rc, ok := r.(io.Closer); ok {
		defer func() { _ = rc.Close() }()
		stop := context.AfterFunc(ctx, func() { _ = rc.Close() })
		defer stop()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, ctxReader{ctx: ctx, r: r}); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
