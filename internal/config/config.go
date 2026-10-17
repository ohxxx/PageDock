// [INPUT]: The current MyGo locale, native name/site/icon/permission settings, and the app user-data directory.
// [OUTPUT]: Validated Config with atomic language, platform-valid name, and settings persistence; older configs follow the system locale.
// [POS]: Config storage boundary; read failures never automatically overwrite the original file.
// [PROTOCOL]: Update this header and internal/config/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	l10n "github.com/ohxxx/PageDock/internal/locale"
	website "github.com/ohxxx/PageDock/internal/site"
)

const (
	Filename         = "config.json"
	maxAppNameLength = 64
)

type Config struct {
	Language           string `json:"language,omitempty"`
	AppName            string `json:"appName,omitempty"`
	IconPNG            string `json:"iconPNG,omitempty"`
	URL                string `json:"url"`
	AllowHTTP          bool   `json:"allowHTTP"`
	AllowNotifications bool   `json:"allowNotifications"`
	AllowCamera        bool   `json:"allowCamera"`
	AllowMicrophone    bool   `json:"allowMicrophone"`
	AllowGeolocation   bool   `json:"allowGeolocation"`
}

func (c Config) Validate() (Config, error) {
	switch c.Language {
	case "", l10n.English, l10n.Chinese:
	default:
		return Config{}, errors.New(l10n.Text("languageInvalid"))
	}
	name, err := ValidateName(c.AppName)
	if err != nil {
		return Config{}, err
	}
	if err := ValidatePlatformName(name); err != nil {
		return Config{}, err
	}
	c.AppName = name
	u, err := website.TargetURL(c.URL, c.AllowHTTP)
	if err != nil {
		return Config{}, err
	}
	if _, err := DecodeIcon(c.IconPNG); err != nil {
		return Config{}, err
	}
	c.URL = u.String()
	return c, nil
}

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > maxAppNameLength {
		return "", fmt.Errorf(l10n.Text("nameTooLong"), maxAppNameLength)
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", errors.New(l10n.Text("nameInvisible"))
		}
	}
	return name, nil
}

func (c Config) DisplayName() string {
	if c.AppName != "" {
		return c.AppName
	}
	return l10n.Text("appName")
}

type Store struct{ Path string }

func (s Store) Load() (Config, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf(l10n.Text("configRead"), err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, errors.New(l10n.Text("configCorrupt"))
	}
	return c.Validate()
}

func (s Store) Save(c Config) error {
	c, err := c.Validate()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf(l10n.Text("configDirectory"), err)
	}
	f, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return fmt.Errorf(l10n.Text("configWrite"), err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf(l10n.Text("configWrite"), err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf(l10n.Text("configSync"), err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.Path); err != nil {
		return fmt.Errorf(l10n.Text("configReplace"), err)
	}
	return nil
}
