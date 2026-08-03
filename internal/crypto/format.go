package crypto

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"pwstore/internal/model"
)

const (
	magic          = "PWSTORE1"
	formatVersion  = 1
	kdfAlgorithm   = 0
	verifierText   = "pwstore-verified"
	maxBlobSize    = 1 << 24
	maxRecordCount = 1 << 20
)

var (
	ErrBadFormat     = errors.New("crypto: not a pwstore vault file")
	ErrWrongPassword = errors.New("crypto: wrong master password")
	ErrUnsupported   = errors.New("crypto: unsupported file version")
)

type Header struct {
	Params Params
}

func EncodeVault(key []byte, p Params, entries []model.Entry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(magic)
	if err := binary.Write(&buf, binary.LittleEndian, uint16(formatVersion)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(kdfAlgorithm)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, p.Memory); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, p.Iter); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(p.Parallel); err != nil {
		return nil, err
	}
	if _, err := buf.Write(p.Salt); err != nil {
		return nil, err
	}

	verifier, err := Seal(key, []byte(verifierText))
	if err != nil {
		return nil, err
	}
	if err := writeBlob(&buf, verifier); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(entries))); err != nil {
		return nil, err
	}
	for _, e := range entries {
		plain, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		sealed, err := Seal(key, plain)
		if err != nil {
			return nil, err
		}
		if err := writeBlob(&buf, sealed); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func DecodeVault(raw []byte, password string) (Params, []model.Entry, error) {
	r := bytes.NewReader(raw)
	m := make([]byte, len(magic))
	if _, err := io.ReadFull(r, m); err != nil || string(m) != magic {
		return Params{}, nil, ErrBadFormat
	}
	var ver, alg uint16
	if err := binary.Read(r, binary.LittleEndian, &ver); err != nil {
		return Params{}, nil, ErrBadFormat
	}
	if ver != formatVersion {
		return Params{}, nil, fmt.Errorf("%w: version %d", ErrUnsupported, ver)
	}
	if err := binary.Read(r, binary.LittleEndian, &alg); err != nil || alg != kdfAlgorithm {
		return Params{}, nil, ErrUnsupported
	}

	p := Params{}
	var err error
	if err := binary.Read(r, binary.LittleEndian, &p.Memory); err != nil {
		return Params{}, nil, ErrBadFormat
	}
	if err := binary.Read(r, binary.LittleEndian, &p.Iter); err != nil {
		return Params{}, nil, ErrBadFormat
	}
	if p.Parallel, err = r.ReadByte(); err != nil {
		return Params{}, nil, ErrBadFormat
	}
	p.Salt = make([]byte, SaltLength)
	if _, err := io.ReadFull(r, p.Salt); err != nil {
		return Params{}, nil, ErrBadFormat
	}

	key, err := p.DeriveKey(password)
	if err != nil {
		return Params{}, nil, err
	}
	verBlob, err := readBlob(r)
	if err != nil {
		return Params{}, nil, ErrBadFormat
	}
	vt, err := Open(key, verBlob)
	if err != nil || string(vt) != verifierText {
		return Params{}, nil, ErrWrongPassword
	}

	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return Params{}, nil, ErrBadFormat
	}
	if n > maxRecordCount {
		return Params{}, nil, ErrBadFormat
	}
	entries := make([]model.Entry, 0, n)
	for i := uint32(0); i < n; i++ {
		blob, err := readBlob(r)
		if err != nil {
			return Params{}, nil, ErrBadFormat
		}
		plain, err := Open(key, blob)
		if err != nil {
			return Params{}, nil, fmt.Errorf("crypto: record %d integrity check failed: %w", i, err)
		}
		var e model.Entry
		if err := json.Unmarshal(plain, &e); err != nil {
			return Params{}, nil, fmt.Errorf("crypto: record %d malformed: %w", i, err)
		}
		entries = append(entries, e)
	}
	return p, entries, nil
}

func writeBlob(buf *bytes.Buffer, blob []byte) error {
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(blob))); err != nil {
		return err
	}
	_, err := buf.Write(blob)
	return err
}

func readBlob(r *bytes.Reader) ([]byte, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	if n > maxBlobSize {
		return nil, errors.New("crypto: blob too large")
	}
	blob := make([]byte, n)
	if _, err := io.ReadFull(r, blob); err != nil {
		return nil, err
	}
	return blob, nil
}
