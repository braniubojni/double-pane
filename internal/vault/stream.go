package vault

import (
	"bufio"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// objectAAD is the associated data for every GCM seal in an object: it binds
// a chunk (or the name field, at the reserved nameChunkIndex) to this exact
// vault and object so chunks cannot be swapped between objects or vaults.
func objectAAD(vaultID, objectID [idSize]byte, chunkIndex uint32) []byte {
	aad := make([]byte, idSize*2+4)
	copy(aad, vaultID[:])
	copy(aad[idSize:], objectID[:])
	binary.LittleEndian.PutUint32(aad[idSize*2:], chunkIndex)
	return aad
}

func writeU32(w io.Writer, v uint32) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	_, err := w.Write(b[:])
	return err
}

func readU32(r io.Reader) (uint32, error) {
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[:]), nil
}

func writeI64(w io.Writer, v int64) error {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	_, err := w.Write(b[:])
	return err
}

func readI64(r io.Reader) (int64, error) {
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}

// rawHeader is the parsed object header before the name is decrypted.
type rawHeader struct {
	nameNonce  []byte
	nameCT     []byte
	origSize   int64
	mtime      int64
	chunkSize  uint32
	chunkCount uint32
}

func readRawHeader(r io.Reader) (rawHeader, error) {
	var h rawHeader
	m := make([]byte, len(magic))
	if _, err := io.ReadFull(r, m); err != nil {
		return h, err
	}
	if string(m) != magic {
		return h, errors.New("vault: bad object magic")
	}
	h.nameNonce = make([]byte, nonceSize)
	if _, err := io.ReadFull(r, h.nameNonce); err != nil {
		return h, err
	}
	nameLen, err := readU32(r)
	if err != nil {
		return h, err
	}
	h.nameCT = make([]byte, nameLen)
	if _, err := io.ReadFull(r, h.nameCT); err != nil {
		return h, err
	}
	if h.origSize, err = readI64(r); err != nil {
		return h, err
	}
	if h.mtime, err = readI64(r); err != nil {
		return h, err
	}
	if h.chunkSize, err = readU32(r); err != nil {
		return h, err
	}
	if h.chunkCount, err = readU32(r); err != nil {
		return h, err
	}
	return h, nil
}

func decryptName(gcm cipher.AEAD, vaultID, objectID [idSize]byte, h rawHeader) (string, error) {
	pt, err := gcm.Open(nil, h.nameNonce, h.nameCT, objectAAD(vaultID, objectID, nameChunkIndex))
	if err != nil {
		return "", ErrInvalidPassword
	}
	return string(pt), nil
}

// writeObject streams r (exactly origSize bytes) into w as one encrypted
// object, chunkSize plaintext bytes (default 1 MiB) at a time. Never buffers
// more than one chunk.
func writeObject(w io.Writer, gcm cipher.AEAD, vaultID, objectID [idSize]byte, relPath string, origSize, mtime int64, r io.Reader) error {
	nameNonce, err := randBytes(nonceSize)
	if err != nil {
		return err
	}
	nameCT := gcm.Seal(nil, nameNonce, []byte(relPath), objectAAD(vaultID, objectID, nameChunkIndex))

	cs := chunkSize
	count := uint32(0)
	if origSize > 0 {
		count = uint32((origSize + int64(cs) - 1) / int64(cs))
	}
	if _, err := io.WriteString(w, magic); err != nil {
		return err
	}
	if _, err := w.Write(nameNonce); err != nil {
		return err
	}
	if err := writeU32(w, uint32(len(nameCT))); err != nil {
		return err
	}
	if _, err := w.Write(nameCT); err != nil {
		return err
	}
	if err := writeI64(w, origSize); err != nil {
		return err
	}
	if err := writeI64(w, mtime); err != nil {
		return err
	}
	if err := writeU32(w, uint32(cs)); err != nil {
		return err
	}
	if err := writeU32(w, count); err != nil {
		return err
	}

	remaining := origSize
	buf := make([]byte, cs)
	for i := uint32(0); i < count; i++ {
		n := cs
		if remaining < int64(n) {
			n = int(remaining)
		}
		if _, err := io.ReadFull(r, buf[:n]); err != nil {
			return err
		}
		nonce, err := randBytes(nonceSize)
		if err != nil {
			return err
		}
		ct := gcm.Seal(nil, nonce, buf[:n], objectAAD(vaultID, objectID, i))
		if _, err := w.Write(nonce); err != nil {
			return err
		}
		if err := writeU32(w, uint32(len(ct))); err != nil {
			return err
		}
		if _, err := w.Write(ct); err != nil {
			return err
		}
		remaining -= int64(n)
	}
	return nil
}

// objectReader streams plaintext out of an object, decrypting one chunk
// (<= chunkSize) at a time. Close releases the underlying file handle.
type objectReader struct {
	f          *os.File
	br         *bufio.Reader
	gcm        cipher.AEAD
	vaultID    [idSize]byte
	objectID   [idSize]byte
	chunkIndex uint32
	chunkCount uint32
	buf        []byte
	pos        int
}

func (r *objectReader) Read(p []byte) (int, error) {
	for r.pos >= len(r.buf) {
		if r.chunkIndex >= r.chunkCount {
			return 0, io.EOF
		}
		if err := r.nextChunk(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}

func (r *objectReader) nextChunk() error {
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(r.br, nonce); err != nil {
		return err
	}
	l, err := readU32(r.br)
	if err != nil {
		return err
	}
	ct := make([]byte, l)
	if _, err := io.ReadFull(r.br, ct); err != nil {
		return err
	}
	pt, err := r.gcm.Open(nil, nonce, ct, objectAAD(r.vaultID, r.objectID, r.chunkIndex))
	if err != nil {
		return ErrInvalidPassword
	}
	r.buf = pt
	r.pos = 0
	r.chunkIndex++
	return nil
}

func (r *objectReader) Close() error {
	if r.f == nil {
		return nil
	}
	return r.f.Close()
}

// openObjectReader opens objPath for streaming decrypt, returning the
// reader plus the decrypted relative path and stat metadata from its header.
func openObjectReader(objPath string, gcm cipher.AEAD, vaultID, objectID [idSize]byte) (*objectReader, string, int64, int64, error) {
	return openObjectReaderAt(objPath, 0, gcm, vaultID, objectID)
}

// openObjectReaderAt is like openObjectReader but starts reading the object
// at a byte offset — used by .dpenc single-file vaults, whose one object
// follows a fixed 512-byte manifest header instead of starting at offset 0.
func openObjectReaderAt(objPath string, offset int64, gcm cipher.AEAD, vaultID, objectID [idSize]byte) (*objectReader, string, int64, int64, error) {
	f, err := os.Open(objPath)
	if err != nil {
		return nil, "", 0, 0, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			_ = f.Close()
			return nil, "", 0, 0, err
		}
	}
	br := bufio.NewReaderSize(f, 64*1024)
	h, err := readRawHeader(br)
	if err != nil {
		_ = f.Close()
		return nil, "", 0, 0, err
	}
	name, err := decryptName(gcm, vaultID, objectID, h)
	if err != nil {
		_ = f.Close()
		return nil, "", 0, 0, err
	}
	return &objectReader{f: f, br: br, gcm: gcm, vaultID: vaultID, objectID: objectID, chunkCount: h.chunkCount}, name, h.origSize, h.mtime, nil
}

// renameObjectHeader rewrites only the encrypted name field of an object,
// copying every chunk byte through unchanged (no re-encryption of payload).
func renameObjectHeader(objPath string, gcm cipher.AEAD, vaultID, objectID [idSize]byte, newRelPath string) error {
	f, err := os.Open(objPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	br := bufio.NewReaderSize(f, 64*1024)
	h, err := readRawHeader(br)
	if err != nil {
		return err
	}

	tmp := objPath + ".tmp-rename"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	nameNonce, err := randBytes(nonceSize)
	if err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	nameCT := gcm.Seal(nil, nameNonce, []byte(newRelPath), objectAAD(vaultID, objectID, nameChunkIndex))
	writeErr := func() error {
		if _, err := io.WriteString(out, magic); err != nil {
			return err
		}
		if _, err := out.Write(nameNonce); err != nil {
			return err
		}
		if err := writeU32(out, uint32(len(nameCT))); err != nil {
			return err
		}
		if _, err := out.Write(nameCT); err != nil {
			return err
		}
		if err := writeI64(out, h.origSize); err != nil {
			return err
		}
		if err := writeI64(out, h.mtime); err != nil {
			return err
		}
		if err := writeU32(out, h.chunkSize); err != nil {
			return err
		}
		if err := writeU32(out, h.chunkCount); err != nil {
			return err
		}
		// Remaining bytes (the chunk records) are untouched ciphertext.
		_, err := io.Copy(out, br)
		return err
	}()
	closeErr := out.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	_ = f.Close() // release before rename over it on Windows
	return os.Rename(tmp, objPath)
}
