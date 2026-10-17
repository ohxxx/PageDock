// [INPUT]: This directory's English/Chinese JSON catalogs and the current MyGo locale.
// [OUTPUT]: Native Catalog registration, supported languages, and translations.
// [POS]: Locale catalog module; callers own switching and persistence.
// [PROTOCOL]: Update the owning AGENTS.md and behavior tests under tests/.
package locale

import (
	"embed"

	"github.com/egoist/mygo/i18n"
	_ "github.com/egoist/mygo/locales/zh-Hans"
)

const (
	English = "en"
	Chinese = "zh-Hans"
)

//go:embed *.json
var languageFiles embed.FS

var Catalog = func() *i18n.Catalog {
	catalog, err := i18n.NewCatalog(languageFiles, English)
	if err != nil {
		panic(err)
	}
	i18n.Register(catalog)
	return catalog
}()

func Text(key string) string { return Catalog.Format(i18n.Locale(), key, nil) }
