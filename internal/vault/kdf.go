package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/argon2"
)

// Wipe zeroes b in place. Call on every DEK/KEK byte slice as soon as it is
// no longer needed (lock, wrap/unwrap failure, process exit).
func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func randBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func randID() ([idSize]byte, error) {
	var id [idSize]byte
	b, err := randBytes(idSize)
	if err != nil {
		return id, err
	}
	copy(id[:], b)
	return id, nil
}

// deriveKEK runs Argon2id over password+salt per p, producing a 32-byte key.
func deriveKEK(password string, salt []byte, p Argon2Params) []byte {
	return argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, uint8(p.Threads), 32)
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// wrapDEK seals dek under kek with AAD=vaultID, returning nonce+ciphertext.
func wrapDEK(kek, dek, vaultID []byte) (nonce, ct []byte, err error) {
	gcm, err := aesGCM(kek)
	if err != nil {
		return nil, nil, err
	}
	nonce, err = randBytes(nonceSize)
	if err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, dek, vaultID), nil
}

// unwrapDEK opens a wrapped DEK. Any failure — wrong password, corrupt
// manifest, tampered ciphertext — collapses to ErrInvalidPassword.
func unwrapDEK(kek, nonce, ct, vaultID []byte) ([]byte, error) {
	gcm, err := aesGCM(kek)
	if err != nil {
		return nil, ErrInvalidPassword
	}
	dek, err := gcm.Open(nil, nonce, ct, vaultID)
	if err != nil {
		return nil, ErrInvalidPassword
	}
	return dek, nil
}

// newManifest generates a fresh vaultID+DEK, wraps the DEK under a
// password-derived KEK, and returns the manifest plus the raw DEK (caller
// must Wipe it when done) and vaultID.
func newManifest(password, hint string) (Manifest, []byte, [idSize]byte, error) {
	salt, err := randBytes(16)
	if err != nil {
		return Manifest{}, nil, [idSize]byte{}, err
	}
	vaultID, err := randID()
	if err != nil {
		return Manifest{}, nil, [idSize]byte{}, err
	}
	dek, err := randBytes(32)
	if err != nil {
		return Manifest{}, nil, [idSize]byte{}, err
	}
	params := DefaultArgon2Params()
	kek := deriveKEK(password, salt, params)
	defer Wipe(kek)
	nonce, ct, err := wrapDEK(kek, dek, vaultID[:])
	if err != nil {
		Wipe(dek)
		return Manifest{}, nil, [idSize]byte{}, err
	}
	man := Manifest{
		Version:      1,
		KDF:          "argon2id",
		Argon2:       params,
		Salt:         base64.StdEncoding.EncodeToString(salt),
		DEKWrapNonce: base64.StdEncoding.EncodeToString(nonce),
		DEKWrapped:   base64.StdEncoding.EncodeToString(ct),
		VaultID:      idHex(vaultID),
		Hint:         hint,
	}
	return man, dek, vaultID, nil
}

// unwrapManifest derives the KEK from password+manifest and unwraps the DEK.
func unwrapManifest(man Manifest, password string) ([]byte, [idSize]byte, error) {
	var vaultID [idSize]byte
	salt, err1 := base64.StdEncoding.DecodeString(man.Salt)
	nonce, err2 := base64.StdEncoding.DecodeString(man.DEKWrapNonce)
	ct, err3 := base64.StdEncoding.DecodeString(man.DEKWrapped)
	vaultID, err4 := parseIDHex(man.VaultID)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return nil, vaultID, ErrInvalidPassword
	}
	kek := deriveKEK(password, salt, man.Argon2)
	defer Wipe(kek)
	dek, err := unwrapDEK(kek, nonce, ct, vaultID[:])
	if err != nil {
		return nil, vaultID, err
	}
	return dek, vaultID, nil
}
