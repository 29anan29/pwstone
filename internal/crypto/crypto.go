package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	KeyLength   = 32
	NonceLength = 12
	SaltLength  = 16
	DefaultMem  = 64 * 1024
	DefaultIter = 3
	DefaultPar  = 4
)

type Params struct {
	Salt     []byte
	Memory   uint32
	Iter     uint32
	Parallel uint8
}

func NewParams() (Params, error) {
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return Params{}, err
	}
	return Params{Salt: salt, Memory: DefaultMem, Iter: DefaultIter, Parallel: DefaultPar}, nil
}

func (p Params) DeriveKey(password string) ([]byte, error) {
	if len(p.Salt) != SaltLength || p.Iter == 0 || p.Memory == 0 || p.Parallel == 0 {
		return nil, errors.New("crypto: invalid kdf params")
	}
	return argon2.IDKey([]byte(password), p.Salt, p.Iter, p.Memory, p.Parallel, KeyLength), nil
}

func Seal(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func Open(key, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("crypto: invalid ciphertext")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
}
