package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

//go:embed templates
var templates embed.FS

// Create writes a new Wahoo application without overwriting existing files.
func Create(directory, module, frameworkPath string) error {
	if _, err := os.Stat(directory); err == nil {
		entries, readErr := os.ReadDir(directory)
		if readErr != nil {
			return fmt.Errorf("inspect %s: %w", directory, readErr)
		}
		if len(entries) > 0 {
			return fmt.Errorf("directory %s is not empty", directory)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", directory, err)
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}
	replace := ""
	if frameworkPath != "" {
		replace = "replace github.com/bjarneo/wahoo => " + filepath.ToSlash(frameworkPath)
	}
	appName := ProjectName(filepath.Base(directory))
	err := fs.WalkDir(templates, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(path, "templates/")
		if strings.HasSuffix(relative, ".tmpl") {
			relative = strings.TrimSuffix(relative, ".tmpl")
		}
		target := filepath.Join(directory, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
		}
		data, err := fs.ReadFile(templates, path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", relative, err)
		}
		content := strings.ReplaceAll(string(data), "__MODULE__", module)
		content = strings.ReplaceAll(content, "__APP_NAME__", appName)
		content = strings.ReplaceAll(content, "__REPLACE_LINE__", replace)
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scaffold project: %w", err)
	}
	return nil
}

// ProjectName converts a directory name into a safe application identifier.
func ProjectName(name string) string {
	var out strings.Builder
	separator := false
	for _, char := range strings.ToLower(name) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			out.WriteRune(char)
			separator = false
			continue
		}
		if out.Len() > 0 && !separator {
			out.WriteByte('-')
			separator = true
		}
	}
	name = strings.Trim(out.String(), "-")
	if name == "" {
		return "app"
	}
	return name
}
