package blocker

// firefoxreg_windows.go — Firefox's policies in the registry too (ROADMAP
// W26): HKLM\SOFTWARE\Policies\Mozilla\Firefox, which every Firefox reads,
// also the Microsoft Store's (whose installation folder can't hold a
// policies.json) and portable ones KC9K doesn't find. KC9K's policies.json
// is mirrored there as Firefox reads its Group Policy: objects as keys,
// lists as keys of values "1", "2"…, booleans and whole numbers as DWORDs,
// text as strings. Only the policies KC9K wrote are taken out again
// (ProgramData\kc9k\firefox-registry.json lists them); others' stay.

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/smattermannen/kc9k/internal/platform"
	"golang.org/x/sys/windows/registry"
)

// firefoxRegKey is Firefox's policy key; a variable for tests.
var firefoxRegKey = `SOFTWARE\Policies\Mozilla\Firefox`

var firefoxRegOwnedPath = filepath.Join(platform.DataDir, "firefox-registry.json")

// syncFirefoxRegistry mirrors policies.json's content (nil: none) into the
// registry.
func syncFirefoxRegistry(policyJSON []byte) error {
	var owned []string
	if data, err := os.ReadFile(firefoxRegOwnedPath); err == nil {
		_ = json.Unmarshal(data, &owned)
	}
	var policies map[string]any
	if policyJSON != nil {
		var root map[string]any
		if err := json.Unmarshal(policyJSON, &root); err != nil {
			return err
		}
		policies, _ = root["policies"].(map[string]any)
	}
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, firefoxRegKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("Firefox policy key: %w", err)
	}
	defer k.Close()
	for _, name := range owned {
		if _, keep := policies[name]; !keep {
			_ = k.DeleteValue(name)
			_ = deleteRegTree(k, name)
		}
	}
	names := make([]string, 0, len(policies))
	for name := range policies {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		_ = k.DeleteValue(name)
		_ = deleteRegTree(k, name)
		if err := writeFirefoxReg(k, name, policies[name]); err != nil {
			return fmt.Errorf("Firefox policy %s: %w", name, err)
		}
	}
	if len(names) == 0 {
		return os.Remove(firefoxRegOwnedPath)
	}
	data, _ := json.Marshal(names)
	return os.WriteFile(firefoxRegOwnedPath, data, 0o600)
}

// writeFirefoxReg writes one value as Firefox's Group Policy reader reads it.
func writeFirefoxReg(k registry.Key, name string, v any) error {
	switch v := v.(type) {
	case bool:
		n := uint32(0)
		if v {
			n = 1
		}
		return k.SetDWordValue(name, n)
	case float64:
		if v == math.Trunc(v) && v >= 0 && v <= math.MaxUint32 {
			return k.SetDWordValue(name, uint32(v))
		}
		return k.SetStringValue(name, strconv.FormatFloat(v, 'f', -1, 64))
	case string:
		return k.SetStringValue(name, v)
	case nil:
		return nil
	case []any:
		sub, _, err := registry.CreateKey(k, name, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		defer sub.Close()
		for i, e := range v {
			if err := writeFirefoxReg(sub, strconv.Itoa(i+1), e); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		sub, _, err := registry.CreateKey(k, name, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		defer sub.Close()
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := writeFirefoxReg(sub, key, v[key]); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("value of type %T", v)
}
