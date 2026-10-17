// [INPUT]: The current MyGo locale, a Windows PE executable, and a validated config icon.
// [OUTPUT]: A PE preserving code and non-icon resources, with the original icon embedded for repeated changes and full restoration.
// [POS]: Windows resource processing testable on any host; reject Authenticode-signed executables.
// [PROTOCOL]: Update internal/appearance/AGENTS.md, icon tests, and platform notes.
package appearance

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"

	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	"github.com/tc-hib/winres"
)

const (
	peIconStateName     = winres.Name("PAGEDOCK_ICON_STATE")
	peIconVersion       = 1
	maxIconEXEBytes     = 256 << 20
	maxPEIconStateBytes = prefs.MaxIconBytes * 4
)

type peIconResource struct {
	Type uint16
	ID   uint16
	Name string
	Lang uint16
	Data []byte
}

type peIconState struct {
	Version  int
	PNGHash  string
	Original []peIconResource
}

func rewritePEIcon(executable []byte, encoded string) ([]byte, bool, error) {
	pngData, err := prefs.DecodeIcon(encoded)
	if err != nil {
		return nil, false, err
	}
	if len(executable) > maxIconEXEBytes {
		return nil, false, errors.New(l10n.Text("windowsIconLarge"))
	}
	file, err := pe.NewFile(bytes.NewReader(executable))
	if err != nil {
		return nil, false, fmt.Errorf(l10n.Text("windowsRead"), err)
	}
	defer file.Close()
	if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 && file.Machine != pe.IMAGE_FILE_MACHINE_ARM64 {
		return nil, false, errors.New(l10n.Text("windowsArchitecture"))
	}
	rs, err := winres.LoadFromEXE(bytes.NewReader(executable))
	if err != nil && !errors.Is(err, winres.ErrNoResources) {
		return nil, false, err
	}
	err = nil
	state := peIconState{Version: peIconVersion}
	stored := rs.Get(winres.RT_RCDATA, peIconStateName, winres.LCIDNeutral)
	if stored != nil {
		if len(stored) > maxPEIconStateBytes || json.Unmarshal(stored, &state) != nil || state.Version != peIconVersion {
			return nil, false, errors.New(l10n.Text("windowsDefaultIconInvalid"))
		}
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(pngData))
	if (encoded == "" && stored == nil) || (encoded != "" && stored != nil && state.PNGHash == hash) {
		return nil, false, nil
	}
	// Resource changes invalidate signatures; never strip a signature or leave an invalid one.
	if header, ok := file.OptionalHeader.(*pe.OptionalHeader64); !ok || header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY] != (pe.DataDirectory{}) {
		return nil, false, errors.New(l10n.Text("windowsIconSigned"))
	}
	var next winres.ResourceSet
	rs.Walk(func(kind, id winres.Identifier, lang uint16, data []byte) bool {
		if kind == winres.RT_ICON || kind == winres.RT_GROUP_ICON {
			if stored == nil {
				r := peIconResource{Type: uint16(kind.(winres.ID)), Lang: lang, Data: data}
				switch id := id.(type) {
				case winres.ID:
					r.ID = uint16(id)
				case winres.Name:
					r.Name = string(id)
				}
				state.Original = append(state.Original, r)
			}
		} else if kind != winres.RT_RCDATA || id != peIconStateName {
			err = next.Set(kind, id, lang, data)
		}
		return err == nil
	})
	if err != nil {
		return nil, false, err
	}
	if encoded == "" {
		for _, r := range state.Original {
			if r.Type != uint16(winres.RT_ICON) && r.Type != uint16(winres.RT_GROUP_ICON) {
				return nil, false, errors.New(l10n.Text("windowsIconSnapshotInvalid"))
			}
			var id winres.Identifier = winres.ID(r.ID)
			if r.Name != "" {
				id = winres.Name(r.Name)
			}
			if err := next.Set(winres.ID(r.Type), id, r.Lang, r.Data); err != nil {
				return nil, false, err
			}
		}
	} else {
		img, err := png.Decode(bytes.NewReader(pngData))
		if err != nil {
			return nil, false, err
		}
		icon, err := winres.NewIconFromResizedImage(img, []int{16, 20, 24, 32, 40, 48, 64, 128, 256})
		if err != nil {
			return nil, false, err
		}
		if err = next.SetIcon(winres.ID(1), icon); err != nil {
			return nil, false, err
		}
		state.PNGHash = hash
		metadata, err := json.Marshal(state)
		if err != nil {
			return nil, false, err
		}
		if len(metadata) > maxPEIconStateBytes {
			return nil, false, errors.New(l10n.Text("windowsIconResourceLarge"))
		}
		if err = next.Set(winres.RT_RCDATA, peIconStateName, winres.LCIDNeutral, metadata); err != nil {
			return nil, false, err
		}
	}
	var out bytes.Buffer
	if err = next.WriteToEXE(&out, bytes.NewReader(executable)); err != nil {
		return nil, false, err
	}
	if err = verifyPEIconRewrite(executable, out.Bytes(), &next); err != nil {
		return nil, false, err
	}
	return out.Bytes(), true, nil
}

func verifyPEIconRewrite(before, after []byte, want *winres.ResourceSet) error {
	old, err := pe.NewFile(bytes.NewReader(before))
	if err != nil {
		return err
	}
	defer old.Close()
	updated, err := pe.NewFile(bytes.NewReader(after))
	if err != nil {
		return err
	}
	defer updated.Close()
	if old.Machine != updated.Machine {
		return errors.New(l10n.Text("windowsArchitectureChanged"))
	}
	// Icon updates may change resources only; preserve code and other data sections byte for byte.
	for _, section := range old.Sections {
		if section.Name == ".rsrc" {
			continue
		}
		next := updated.Section(section.Name)
		if next == nil {
			return fmt.Errorf(l10n.Text("windowsSectionLost"), section.Name)
		}
		a, ea := section.Data()
		b, eb := next.Data()
		if ea != nil || eb != nil || !bytes.Equal(a, b) {
			return fmt.Errorf(l10n.Text("windowsSectionChanged"), section.Name)
		}
	}
	actual, err := winres.LoadFromEXE(bytes.NewReader(after))
	if err != nil && !errors.Is(err, winres.ErrNoResources) {
		return err
	}
	if actual.Count() != want.Count() {
		return errors.New(l10n.Text("windowsResourceCount"))
	}
	err = nil
	want.Walk(func(kind, id winres.Identifier, lang uint16, data []byte) bool {
		if !bytes.Equal(data, actual.Get(kind, id, lang)) {
			err = errors.New(l10n.Text("windowsResourceValidation"))
		}
		return err == nil
	})
	return err
}
