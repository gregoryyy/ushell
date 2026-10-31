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

	"github.com/gregoryyy/ushell/ush/go/internal/providers"
)

const (
	keychainService = "ush"
	keychainAccount = "anthropic"
)

// Security is macOS's keychain tool; a variable so tests can point it elsewhere.
var Security = "/usr/bin/security"

var (
	keyEnv     = providers.KeyEnv
	keyPattern = regexp.MustCompile(`^[A-Za-z0-9_\-]{20,300}$`)
)

// KeyStore finds and stores the Anthropic API key: environment, macOS keychain, 0600 file.
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

// Resolve returns the key and where it came from; "" and "none" if there is none.
func (k *KeyStore) Resolve() (string, string, error) {
	for _, name := range keyEnv {
		if v := strings.TrimSpace(k.Getenv(name)); v != "" {
			return v, fmt.Sprintf("environment (%s)", name), nil
		}
	}
	if k.keychainAvailable() {
		if out, err := k.security("", "find-generic-password", "-s", keychainService, "-a", keychainAccount, "-w"); err == nil && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out), "macOS keychain", nil
		}
	}
	text, err := ReadPrivate(k.CredentialsPath(), k.ID.OwnerUID, 0)
	if err != nil {
		return "", "none", err
	}
	if text != "" {
		var data map[string]any
		if json.Unmarshal([]byte(text), &data) != nil {
			return "", "none", Errorf("%s is not valid JSON", k.CredentialsPath())
		}
		if key, _ := data["anthropic_api_key"].(string); key != "" {
			return key, fmt.Sprintf("file (%s)", k.CredentialsPath()), nil
		}
	}
	return "", "none", nil
}

// Store saves the key in the keychain (preferred, on macOS) or the 0600 file; returns where.
func (k *KeyStore) Store(key string, preferKeychain bool) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", Errorf("that does not look like an API key")
	}
	if preferKeychain && k.keychainAvailable() {
		command := fmt.Sprintf("add-generic-password -U -s %s -a %s -l \"ush API key\" -w \"%s\"\n",
			keychainService, keychainAccount, key)
		if _, err := k.security(command, "-i"); err == nil { // stdin keeps the key out of argv
			k.removeFile()
			return "macOS keychain", nil
		}
	}
	if _, err := privateDir(k.ID.ConfigDir(), k.ID); err != nil {
		return "", err
	}
	data, _ := json.Marshal(map[string]string{"anthropic_api_key": key})
	if err := WritePrivate(k.CredentialsPath(), append(data, '\n'), k.ID); err != nil {
		return "", err
	}
	return fmt.Sprintf("file (%s)", k.CredentialsPath()), nil
}

// Clear removes the key from the keychain and the file; returns what was removed.
func (k *KeyStore) Clear() []string {
	var removed []string
	if k.keychainAvailable() {
		if _, err := k.security("", "delete-generic-password", "-s", keychainService, "-a", keychainAccount); err == nil {
			removed = append(removed, "macOS keychain")
		}
	}
	if k.removeFile() {
		removed = append(removed, k.CredentialsPath())
	}
	return removed
}

func (k *KeyStore) removeFile() bool {
	return os.Remove(k.CredentialsPath()) == nil
}

// Mask shows the start and end of a key only.
func Mask(key string) string {
	if len([]rune(key)) <= 16 {
		return "…"
	}
	r := []rune(key)
	return string(r[:7]) + "…" + string(r[len(r)-4:])
}
