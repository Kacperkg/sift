package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Store struct{ path, service string }

func New(path, service string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	return &Store{path: path, service: service}, nil
}

func RandomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Store) Save(v any) error {
	plain, err := json.Marshal(v)
	if err != nil {
		return err
	}
	key, err := s.encryptionKey()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nonce, nonce, plain, []byte("job-inbox-v1"))
	return os.WriteFile(s.path, sealed, 0600)
}

func (s *Store) Load(v any) error {
	sealed, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	key, err := s.encryptionKey()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(sealed) < gcm.NonceSize() {
		return errors.New("encrypted data is invalid")
	}
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], []byte("job-inbox-v1"))
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, v)
}

func (s *Store) encryptionKey() ([]byte, error) {
	encoded, err := GetSecret(s.service, "data-key")
	if err == nil {
		return base64.RawStdEncoding.DecodeString(encoded)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := SetSecret(s.service, "data-key", base64.RawStdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}

func GetSecret(service, account string) (string, error) {
	out, err := exec.Command("/usr/bin/security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func SetSecret(service, account, value string) error {
	cmd := exec.Command("/usr/bin/security", "add-generic-password", "-U", "-s", service, "-a", account, "-w", value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Keychain error: %w: %s", err, out)
	}
	return nil
}
