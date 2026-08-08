package scaffold

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

//go:embed all:templates
var templates embed.FS

// Module is an optional application capability.
type Module string

const (
	// ModuleAuth adds account route stubs.
	ModuleAuth Module = "auth"
	// ModuleSSE adds a server-sent event endpoint.
	ModuleSSE Module = "sse"
	// ModuleWebSocket adds a development WebSocket endpoint.
	ModuleWebSocket Module = "websocket"
)

const moduleMarker = "\t// wahoo:modules\n"

type moduleSpec struct {
	template     string
	target       string
	registration string
	description  string
}

var moduleSpecs = map[Module]moduleSpec{
	ModuleAuth: {
		template:     "templates/modules/auth.go.tmpl",
		target:       "app/auth.go",
		registration: "\tregisterAuth(runtime)\n",
		description:  "account route stubs",
	},
	ModuleSSE: {
		template:     "templates/modules/sse.go.tmpl",
		target:       "app/sse.go",
		registration: "\tregisterSSE(runtime)\n",
		description:  "server-sent events",
	},
	ModuleWebSocket: {
		template:     "templates/modules/websocket.go.tmpl",
		target:       "app/websocket.go",
		registration: "\tregisterWebSocket(runtime)\n",
		description:  "WebSocket endpoint",
	},
}

type manifest struct {
	Modules []Module `json:"modules"`
}

// Modules returns the modules that a Wahoo project can install.
func Modules() []Module {
	return []Module{ModuleAuth, ModuleSSE, ModuleWebSocket}
}

// ParseModules validates comma-separated module values and removes duplicates.
func ParseModules(values []string) ([]Module, error) {
	var modules []Module
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			module := Module(strings.TrimSpace(part))
			if module == "" {
				continue
			}
			if _, ok := moduleSpecs[module]; !ok {
				return nil, fmt.Errorf("unknown module %q", module)
			}
			if !contains(modules, module) {
				modules = append(modules, module)
			}
		}
	}
	return modules, nil
}

// Create writes a new Wahoo application without overwriting existing files.
func Create(directory, module, frameworkPath string, modules ...Module) error {
	modules, err := ParseModules(moduleStrings(modules))
	if err != nil {
		return err
	}
	if info, err := os.Lstat(directory); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("project directory %s must not be a symbolic link", directory)
		}
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
	err = fs.WalkDir(templates, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(path, "templates/")
		if strings.HasPrefix(relative, "modules/") {
			return nil
		}
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
		content = strings.ReplaceAll(content, "__MODULES__", moduleList(modules))
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scaffold project: %w", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open project root: %w", err)
	}
	defer root.Close()
	if err := writeManifest(root, manifest{}); err != nil {
		return err
	}
	if len(modules) == 0 {
		return nil
	}
	return Add(directory, modules...)
}

// Add installs modules in an existing Wahoo project.
func Add(directory string, modules ...Module) error {
	modules, err := ParseModules(moduleStrings(modules))
	if err != nil {
		return err
	}
	if len(modules) == 0 {
		return errors.New("at least one module is required")
	}

	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("inspect project directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("project directory %s must not be a symbolic link", directory)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open project root: %w", err)
	}
	defer root.Close()
	if err := rejectManagedSymlinks(root); err != nil {
		return err
	}

	project, err := readManifest(root)
	if err != nil {
		return err
	}
	routes, err := root.ReadFile("app/routes.go")
	if err != nil {
		return fmt.Errorf("read application routes: %w", err)
	}
	if !strings.Contains(string(routes), moduleMarker) {
		return errors.New("application routes do not contain the Wahoo module marker")
	}

	var additions []Module
	for _, module := range modules {
		if contains(project.Modules, module) {
			continue
		}
		spec := moduleSpecs[module]
		if _, err := root.Lstat(spec.target); err == nil {
			return fmt.Errorf("module %q target already exists: %s", module, spec.target)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect module %q target: %w", module, err)
		}
		additions = append(additions, module)
	}
	if len(additions) == 0 {
		return nil
	}

	for _, module := range additions {
		spec := moduleSpecs[module]
		data, err := fs.ReadFile(templates, spec.template)
		if err != nil {
			return fmt.Errorf("read module %q template: %w", module, err)
		}
		if err := writeNewFile(root, spec.target, data, 0o644); err != nil {
			return fmt.Errorf("write module %q: %w", module, err)
		}
		routes = []byte(strings.Replace(string(routes), moduleMarker, spec.registration+moduleMarker, 1))
		project.Modules = append(project.Modules, module)
	}
	if err := writeAtomic(root, "app/routes.go", routes, 0o644); err != nil {
		return fmt.Errorf("write application routes: %w", err)
	}
	return writeManifest(root, project)
}

func rejectManagedSymlinks(root *os.Root) error {
	for _, path := range []string{"app", ".wahoo", "app/routes.go", ".wahoo/modules.json"} {
		info, err := root.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect managed path %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed path %s must not be a symbolic link", path)
		}
	}
	return nil
}

func readManifest(root *os.Root) (manifest, error) {
	data, err := root.ReadFile(".wahoo/modules.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return manifest{}, errors.New("not a Wahoo project: .wahoo/modules.json is missing")
		}
		return manifest{}, fmt.Errorf("read module manifest: %w", err)
	}
	var project manifest
	if err := json.Unmarshal(data, &project); err != nil {
		return manifest{}, fmt.Errorf("decode module manifest: %w", err)
	}
	modules, err := ParseModules(moduleStrings(project.Modules))
	if err != nil {
		return manifest{}, fmt.Errorf("validate module manifest: %w", err)
	}
	project.Modules = modules
	return project, nil
}

func writeManifest(root *os.Root, project manifest) error {
	if project.Modules == nil {
		project.Modules = []Module{}
	}
	if err := root.MkdirAll(".wahoo", 0o755); err != nil {
		return fmt.Errorf("create module manifest directory: %w", err)
	}
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("encode module manifest: %w", err)
	}
	data = append(data, '\n')
	if err := writeAtomic(root, ".wahoo/modules.json", data, 0o644); err != nil {
		return fmt.Errorf("write module manifest: %w", err)
	}
	return nil
}

func writeNewFile(root *os.Root, path string, data []byte, perm fs.FileMode) error {
	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer file.Close()
	if written, err := file.Write(data); err != nil {
		return err
	} else if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func writeAtomic(root *os.Root, path string, data []byte, perm fs.FileMode) error {
	temporary := path + ".wahoo.tmp"
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if written, err := file.Write(data); err != nil {
		file.Close()
		return err
	} else if written != len(data) {
		file.Close()
		return io.ErrShortWrite
	}
	if err := file.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, path)
}

func moduleList(modules []Module) string {
	if len(modules) == 0 {
		return "- Core only. Add optional modules with `wahoo add ./project <module>`."
	}
	lines := make([]string, 0, len(modules))
	for _, module := range modules {
		lines = append(lines, "- `"+string(module)+"`: "+moduleSpecs[module].description+".")
	}
	return strings.Join(lines, "\n")
}

func moduleStrings(modules []Module) []string {
	values := make([]string, len(modules))
	for i, module := range modules {
		values[i] = string(module)
	}
	return values
}

func contains(modules []Module, target Module) bool {
	for _, module := range modules {
		if module == target {
			return true
		}
	}
	return false
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
