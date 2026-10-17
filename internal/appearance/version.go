// [INPUT]: The current MyGo locale, a Windows PE, and a validated app name, using existing icon resource processing.
// [OUTPUT]: Multilingual product name, file description, and Shell display strings, with full original version-resource restoration.
// [POS]: Name resource transactions testable on any host; preserve code, application identity, and file paths.
// [PROTOCOL]: Update internal/appearance/AGENTS.md, name behavior tests, and platform notes.
package appearance

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
	"unicode/utf16"

	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

const (
	peNameStateName    = winres.Name("PAGEDOCK_NAME_STATE")
	peNameStateVersion = 1
	DisplayNameID      = 0xfff0
	peDisplayNameBlock = winres.ID(DisplayNameID/16 + 1)
	MaxNameStateBytes  = 1 << 20
)

type peNameState struct {
	Version  int
	Name     string
	Original []peIconResource
}

func RewritePE(executable []byte, config prefs.Config) ([]byte, bool, error) {
	icon, iconChanged, err := rewritePEIcon(executable, config.IconPNG)
	if err != nil {
		return nil, false, err
	}
	if iconChanged {
		executable = icon
	}
	named, nameChanged, err := rewritePEName(executable, config.AppName)
	if err != nil {
		return nil, false, err
	}
	if nameChanged {
		return named, true, nil
	}
	if iconChanged {
		return icon, true, nil
	}
	return nil, false, nil
}

func rewritePEName(executable []byte, name string) ([]byte, bool, error) {
	name, err := prefs.ValidateName(name)
	if err != nil {
		return nil, false, err
	}
	if len(executable) > maxIconEXEBytes {
		return nil, false, errors.New(l10n.Text("windowsNameLarge"))
	}
	file, err := pe.NewFile(bytes.NewReader(executable))
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 && file.Machine != pe.IMAGE_FILE_MACHINE_ARM64 {
		return nil, false, errors.New(l10n.Text("windowsArchitecture"))
	}
	rs, err := winres.LoadFromEXE(bytes.NewReader(executable))
	if err != nil && !errors.Is(err, winres.ErrNoResources) {
		return nil, false, err
	}
	state := peNameState{Version: peNameStateVersion}
	stored := rs.Get(winres.RT_RCDATA, peNameStateName, winres.LCIDNeutral)
	if stored != nil && (len(stored) > MaxNameStateBytes || json.Unmarshal(stored, &state) != nil || state.Version != peNameStateVersion) {
		return nil, false, errors.New(l10n.Text("windowsDefaultNameInvalid"))
	}
	if (name == "" && stored == nil) || (stored != nil && state.Name == name) {
		return nil, false, nil
	}
	if header, ok := file.OptionalHeader.(*pe.OptionalHeader64); !ok || header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY] != (pe.DataDirectory{}) {
		return nil, false, errors.New(l10n.Text("windowsNameSigned"))
	}
	var next winres.ResourceSet
	err = nil
	rs.Walk(func(kind, id winres.Identifier, lang uint16, data []byte) bool {
		switch {
		case kind == winres.RT_VERSION:
			if stored == nil {
				resource := peIconResource{Type: uint16(winres.RT_VERSION), Lang: lang, Data: data}
				switch id := id.(type) {
				case winres.ID:
					resource.ID = uint16(id)
				case winres.Name:
					resource.Name = string(id)
				}
				state.Original = append(state.Original, resource)
			}
		case kind == winres.RT_STRING && id == peDisplayNameBlock:
			if stored == nil {
				err = errors.New(l10n.Text("windowsNameResourceUsed"))
			}
		case kind == winres.RT_RCDATA && id == peNameStateName:
		default:
			err = next.Set(kind, id, lang, data)
		}
		return err == nil
	})
	if err != nil {
		return nil, false, err
	}
	for _, r := range state.Original {
		if r.Type != uint16(winres.RT_VERSION) {
			return nil, false, errors.New(l10n.Text("windowsNameSnapshotInvalid"))
		}
		var id winres.Identifier = winres.ID(r.ID)
		if r.Name != "" {
			id = winres.Name(r.Name)
		}
		data := r.Data
		if name != "" {
			info, err := version.FromBytes(data)
			if err != nil {
				return nil, false, err
			}
			if err = setPEVersionName(info, name); err != nil {
				return nil, false, err
			}
			data = info.Bytes()
		}
		if err = next.Set(winres.RT_VERSION, id, r.Lang, data); err != nil {
			return nil, false, err
		}
	}
	if name != "" {
		if len(state.Original) == 0 {
			info := &version.Info{}
			if err = setPEVersionName(info, name); err != nil {
				return nil, false, err
			}
			if err = next.Set(winres.RT_VERSION, winres.ID(1), winres.LCIDNeutral, info.Bytes()); err != nil {
				return nil, false, err
			}
		}
		if err = next.Set(winres.RT_STRING, peDisplayNameBlock, winres.LCIDNeutral, peNameString(name)); err != nil {
			return nil, false, err
		}
		state.Name = name
		data, err := json.Marshal(state)
		if err != nil {
			return nil, false, err
		}
		if len(data) > MaxNameStateBytes {
			return nil, false, errors.New(l10n.Text("windowsNameResourceLarge"))
		}
		if err = next.Set(winres.RT_RCDATA, peNameStateName, winres.LCIDNeutral, data); err != nil {
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

func setPEVersionName(info *version.Info, name string) error {
	languages := info.Table()
	if len(languages) == 0 {
		languages = version.LangTable{version.LangNeutral: nil}
	}
	for lang := range languages {
		for _, key := range []string{version.ProductName, version.FileDescription} {
			if err := info.Set(lang, key, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func peNameString(name string) []byte {
	chars := utf16.Encode([]rune(name))
	// Each RT_STRING block has exactly 16 entries; this app uses the first ID in the block.
	data := make([]byte, 2*(16+len(chars)))
	binary.LittleEndian.PutUint16(data, uint16(len(chars)))
	for i, c := range chars {
		binary.LittleEndian.PutUint16(data[2+2*i:], c)
	}
	return data
}

func PEIdentity(executable []byte) (name, identifier string, err error) {
	rs, err := winres.LoadFromEXE(bytes.NewReader(executable))
	if errors.Is(err, winres.ErrNoResources) {
		return InternalName, "", nil
	}
	if err != nil {
		return "", "", err
	}
	rs.Walk(func(kind, _ winres.Identifier, _ uint16, data []byte) bool {
		if kind != winres.RT_VERSION {
			return true
		}
		info, readErr := version.FromBytes(data)
		if readErr != nil {
			err = readErr
			return false
		}
		tables := info.Table()
		languages := make([]int, 0, len(tables))
		for lang := range tables {
			languages = append(languages, int(lang))
		}
		sort.Ints(languages)
		for _, lang := range languages {
			table := tables[uint16(lang)]
			if table == nil {
				continue
			}
			if value := (*table)[version.ProductName]; name == "" && value != "" {
				name = value
			}
			if value := (*table)["MyGoIdentifier"]; value != "" {
				if identifier != "" && identifier != value {
					err = errors.New(l10n.Text("windowsIdentityMismatch"))
					return false
				}
				identifier = value
			}
		}
		return true
	})
	if name == "" {
		name = InternalName
	}
	return name, identifier, err
}
