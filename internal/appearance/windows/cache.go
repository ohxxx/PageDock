// [INPUT]: Validated appearance settings and the existing transactional apply/save operations.
// [OUTPUT]: Serialized appearance synchronization and ordinary config saves.
// [POS]: Portable Windows save gate; native EXE/Shell work stays in apply.go.
// [PROTOCOL]: Update this header, the Windows AGENTS.md, and cache behavior tests.
package windows

import (
	"sync"

	prefs "github.com/ohxxx/PageDock/internal/config"
)

type appearanceCache struct {
	mu         sync.Mutex
	ready      bool
	name, icon string
}

func (c *appearanceCache) apply(config prefs.Config, save func() error, synchronize func(prefs.Config, func() error) (bool, error)) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ready && c.name == config.AppName && c.icon == config.IconPNG {
		return false, save()
	}
	changed, err := synchronize(config, save)
	// An unsuccessful transaction may also have failed to roll back; retry native synchronization.
	c.ready = err == nil
	if c.ready {
		c.name, c.icon = config.AppName, config.IconPNG
	}
	return changed, err
}
