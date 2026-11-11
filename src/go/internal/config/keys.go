package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gregoryyy/ushell/src/go/internal/providers"
)

const keychainService = "ush"

// Security is macOS's keychain tool; a variable so tests can point it elsewhere.
var Security = "/usr/bin/security"

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_\-]{20,300}$`)

// KeyStore finds and stores the API keys, one per provider: environment, macOS keychain, 0600 file.
type KeyStore struct {
	ID     Identity
	Getenv func(string) string
	GOOS   string
}

// NewKeyStore uses the process environment.
func NewKeyStore(id Identity) *KeyStore {
	return &KeyStore{ID: id, Getenv: os.Getenv, GOOS: runtime.GOOS}
}

// CredentialsPath is the key file's path.
func (k *KeyStore) CredentialsPath() string { return filepath.Join(k.ID.ConfigDir(), "credentials") }

func (k *KeyStore) keychainAvailable() bool {
	if k.GOOS != "darwin" {
		return false
	}
	info, err := os.Stat(Security)
	return err == nil && info.Mode()&0o111 != 0
}

func (k *KeyStore) security(stdin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, Security, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	if k.ID.IsRoot() && k.ID.ViaSudo() {
		// The invoking user's login keychain.
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(k.ID.OwnerUID), Gid: uint32(ownerGID(k.ID))}}
		cmd.Env = []string{"HOME=" + k.ID.OwnerHome, "PATH=/usr/bin:/bin"}
	}
	out, err := cmd.Output()
	return string(out), err
}

func ownerGID(id Identity) int {
	if info, err := os.Stat(id.OwnerHome); err == nil {
		return int(info.Sys().(*syscall.Stat_t).Gid)
	}
	return 20 // staff
}

// Resolve returns a provider's key and where it came from: its variables, the keychain (account =
// provider), or the credentials file ("<provider>_api_key"); "" and "none" if there is none.
func (k *KeyStore) Resolve(provider string) (string, string, error) {
	for _, name := range providers.MustGet(provider).KeyEnv {
		if v := strings.TrimSpace(k.Getenv(name)); v != "" {
			return v, fmt.Sprintf("environment (%s)", name), nil
		}
	}
	if k.keychainAvailable() {
		if out, err := k.security("", "find-generic-password", "-s", keychainService, "-a", provider, "-w"); err == nil && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out), "macOS keychain", nil
		}
	}
	data, err := k.fileKeys()
	if err != nil {
		return "", "none", err
	}
	if key, _ := data[provider+"_api_key"].(string); key != "" {
		return key, fmt.Sprintf("file (%s)", k.CredentialsPath()), nil
	}
	return "", "none", nil
}

// fileKeys reads the credentials file: one "<provider>_api_key" entry per provider.
func (k *KeyStore) fileKeys() (map[string]any, error) {
	text, err := ReadPrivate(k.CredentialsPath(), k.ID.OwnerUID, 0)
	if err != nil {
		return nil, err
	}
	data := map[string]any{}
	if text == "" {
		return data, nil
	}
	if json.Unmarshal([]byte(text), &data) != nil || data == nil {
		return nil, Errorf("%s is not valid JSON", k.CredentialsPath())
	}
	return data, nil
}

// Store saves a provider's key in the keychain (preferred, on macOS) or the 0600 file, which
// keeps the other providers' keys; returns where.
func (k *KeyStore) Store(key string, preferKeychain bool, provider string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", Errorf("that does not look like an API key")
	}
	if preferKeychain && k.keychainAvailable() {
		command := fmt.Sprintf("add-generic-password -U -s %s -a %s -l \"ush API key (%s)\" -w \"%s\"\n",
			keychainService, provider, providers.MustGet(provider).Label, key)
		if _, err := k.security(command, "-i"); err == nil { // stdin keeps the key out of argv
			k.removeFileKey(provider)
			return "macOS keychain", nil
		}
	}
	keys, err := k.fileKeys()
	if err != nil {
		return "", err
	}
	keys[provider+"_api_key"] = key
	if _, err := privateDir(k.ID.ConfigDir(), k.ID); err != nil {
		return "", err
	}
	if err := k.writeKeys(keys); err != nil {
		return "", err
	}
	return fmt.Sprintf("file (%s)", k.CredentialsPath()), nil
}

func (k *KeyStore) writeKeys(keys map[string]any) error {
	data, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	return WritePrivate(k.CredentialsPath(), append(data, '\n'), k.ID)
}

// Clear removes a provider's key from the keychain and the file; returns what was removed.
func (k *KeyStore) Clear(provider string) []string {
	var removed []string
	if k.keychainAvailable() {
		if _, err := k.security("", "delete-generic-password", "-s", keychainService, "-a", provider); err == nil {
			removed = append(removed, "macOS keychain")
		}
	}
	if k.removeFileKey(provider) {
		removed = append(removed, k.CredentialsPath())
	}
	return removed
}

// removeFileKey removes one provider's key from the credentials file; the file goes with the last.
func (k *KeyStore) removeFileKey(provider string) bool {
	keys, err := k.fileKeys()
	if err != nil {
		keys = map[string]any{}
	}
	if _, ok := keys[provider+"_api_key"]; !ok {
		return false
	}
	delete(keys, provider+"_api_key")
	if len(keys) > 0 {
		_ = k.writeKeys(keys)
	} else {
		_ = os.Remove(k.CredentialsPath())
	}
	return true
}

// Mask shows the start and end of a key only.
func Mask(key string) string {
	if len([]rune(key)) <= 16 {
		return "…"
	}
	r := []rune(key)
	return string(r[:7]) + "…" + string(r[len(r)-4:])
}
