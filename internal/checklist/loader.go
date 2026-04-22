package checklist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a checklist from a file path or a named checklist in
// the user's ~/.preflt directory.
func Load(nameOrPath string) (*Checklist, error) {
	path := resolvePath(nameOrPath)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading checklist %q: %w", path, err)
	}

	var cl Checklist
	if err := yaml.Unmarshal(data, &cl); err != nil {
		return nil, fmt.Errorf("parsing checklist %q: %w", path, err)
	}

	applyDefaults(&cl)

	if err := cl.Validate(); err != nil {
		return nil, fmt.Errorf("invalid checklist %q: %w", path, err)
	}

	return &cl, nil
}

// resolvePath determines the actual file path for the given name or path.
func resolvePath(nameOrPath string) string {
	if strings.Contains(nameOrPath, "/") ||
		strings.HasSuffix(nameOrPath, ".yaml") ||
		strings.HasSuffix(nameOrPath, ".yml") {
		return nameOrPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".preflt", nameOrPath+".yaml")
}

// applyDefaults fills in zero-value fields with sensible defaults.
// IDs are assigned using a global counter across all phases so they are
// always unique within the checklist.
func applyDefaults(cl *Checklist) {
	if cl.Type == "" {
		cl.Type = TypeNormal
	}

	globalIdx := 0

	applyToItems := func(items []Item) []Item {
		for i := range items {
			if items[i].Type == "" {
				items[i].Type = ItemDo
			}
			if items[i].ID == "" {
				items[i].ID = fmt.Sprintf("item-%d", globalIdx)
			}
			globalIdx++
		}
		return items
	}

	if len(cl.Phases) > 0 {
		for p := range cl.Phases {
			cl.Phases[p].Items = applyToItems(cl.Phases[p].Items)
		}
	} else {
		cl.Items = applyToItems(cl.Items)
	}
}

// ListPaths returns all checklist YAML file paths found in ~/.preflt and the
// current working directory. Duplicates (by base name) are removed.
func ListPaths() ([]string, error) {
	seen := make(map[string]bool)
	var paths []string

	addDir := func(dir string) {
		for _, pattern := range []string{"*.yaml", "*.yml"} {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				continue
			}
			for _, m := range matches {
				abs, err := filepath.Abs(m)
				if err != nil {
					abs = m
				}
				key := filepath.Base(abs)
				if !seen[key] {
					seen[key] = true
					paths = append(paths, abs)
				}
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		addDir(filepath.Join(home, ".preflt"))
	}

	if cwd, err := os.Getwd(); err == nil {
		addDir(cwd)
	}

	return paths, nil
}
