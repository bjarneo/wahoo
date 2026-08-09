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
	"regexp"
	"strings"
	"unicode"
)

//go:embed all:templates
var templates embed.FS

// Module is an optional application capability.
type Module string

const (
	// FrameworkVersion is the Wahoo module version used by newly generated projects.
	FrameworkVersion = "v0.4.0"

	// ModuleAuth adds account route stubs.
	ModuleAuth Module = "auth"
	// ModuleSSE adds a server-sent event endpoint.
	ModuleSSE Module = "sse"
	// ModuleWebSocket adds a development WebSocket endpoint.
	ModuleWebSocket Module = "websocket"
	// ModuleOpenAPI adds an application-owned OpenAPI document.
	ModuleOpenAPI Module = "openapi"
	// ModuleJobs adds a worker command and job processing seams.
	ModuleJobs Module = "jobs"
	// ModuleUploads adds an upload configuration stub.
	ModuleUploads Module = "uploads"
	// ModuleMail adds a mail configuration stub.
	ModuleMail Module = "mail"
	// ModuleAudit adds an audit configuration stub.
	ModuleAudit Module = "audit"
	// ModuleWebhooks adds a webhook configuration stub.
	ModuleWebhooks Module = "webhooks"
	// ModuleEntitlements adds an entitlement configuration stub.
	ModuleEntitlements Module = "entitlements"
	// ModuleBilling adds a billing configuration stub.
	ModuleBilling Module = "billing"
)

const moduleMarker = "\t// wahoo:modules\n"

const (
	metadataFormatVersion = 1
	metadataPath          = ".wahoo/project.json"
	legacyManifestPath    = ".wahoo/modules.json"
)

type moduleSpec struct {
	files        []moduleFile
	registration string
	description  string
}

type moduleFile struct {
	template string
	target   string
}

var moduleSpecs = map[Module]moduleSpec{
	ModuleAuth: {
		files:        []moduleFile{{template: "templates/modules/auth.go.tmpl", target: "app/auth.go"}},
		registration: "\tregisterAuth(runtime)\n",
		description:  "account route stubs",
	},
	ModuleSSE: {
		files:        []moduleFile{{template: "templates/modules/sse.go.tmpl", target: "app/sse.go"}},
		registration: "\tregisterSSE(runtime)\n",
		description:  "server-sent events",
	},
	ModuleWebSocket: {
		files:        []moduleFile{{template: "templates/modules/websocket.go.tmpl", target: "app/websocket.go"}},
		registration: "\tregisterWebSocket(runtime)\n",
		description:  "WebSocket endpoint",
	},
	ModuleOpenAPI: {
		files: []moduleFile{
			{template: "templates/modules/openapi.go.tmpl", target: "app/openapi.go"},
			{template: "templates/modules/openapi.json.tmpl", target: "app/openapi.json"},
		},
		registration: "\tregisterOpenAPI(runtime)\n",
		description:  "application-owned OpenAPI document",
	},
	ModuleJobs: {
		files: []moduleFile{
			{template: "templates/modules/jobs.go.tmpl", target: "app/jobs.go"},
			{template: "templates/modules/worker-main.go.tmpl", target: "cmd/worker/main.go"},
		},
		description: "worker command and job seams",
	},
	ModuleUploads: {
		files:        []moduleFile{{template: "templates/modules/uploads.go.tmpl", target: "app/uploads.go"}},
		registration: "\tregisterUploads(runtime)\n",
		description:  "upload configuration stub",
	},
	ModuleMail: {
		files:        []moduleFile{{template: "templates/modules/mail.go.tmpl", target: "app/mail.go"}},
		registration: "\tregisterMail(runtime)\n",
		description:  "mail configuration stub",
	},
	ModuleAudit: {
		files:        []moduleFile{{template: "templates/modules/audit.go.tmpl", target: "app/audit.go"}},
		registration: "\tregisterAudit(runtime)\n",
		description:  "audit configuration stub",
	},
	ModuleWebhooks: {
		files:        []moduleFile{{template: "templates/modules/webhooks.go.tmpl", target: "app/webhooks.go"}},
		registration: "\tregisterWebhooks(runtime)\n",
		description:  "webhook configuration stub",
	},
	ModuleEntitlements: {
		files:        []moduleFile{{template: "templates/modules/entitlements.go.tmpl", target: "app/entitlements.go"}},
		registration: "\tregisterEntitlements(runtime)\n",
		description:  "entitlement configuration stub",
	},
	ModuleBilling: {
		files:        []moduleFile{{template: "templates/modules/billing.go.tmpl", target: "app/billing.go"}},
		registration: "\tregisterBilling(runtime)\n",
		description:  "billing configuration stub",
	},
}

type manifest struct {
	FormatVersion    int      `json:"format_version"`
	FrameworkVersion string   `json:"framework_version"`
	Modules          []Module `json:"modules"`
}

// Modules returns the modules that a Wahoo project can install.
func Modules() []Module {
	return []Module{
		ModuleAuth,
		ModuleSSE,
		ModuleWebSocket,
		ModuleOpenAPI,
		ModuleJobs,
		ModuleUploads,
		ModuleMail,
		ModuleAudit,
		ModuleWebhooks,
		ModuleEntitlements,
		ModuleBilling,
	}
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
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open project root: %w", err)
	}
	defer root.Close()

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
		parent := filepath.ToSlash(filepath.Dir(relative))
		if parent != "." {
			if err := root.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", parent, err)
			}
		}
		data, err := fs.ReadFile(templates, path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", relative, err)
		}
		content := renderTemplate(data, module, appName, replace, modules)
		if err := writeNewFile(root, filepath.ToSlash(relative), content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", relative, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scaffold project: %w", err)
	}
	if err := writeManifest(root, manifest{FrameworkVersion: FrameworkVersion}); err != nil {
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
	if err := rejectManagedSymlinks(root, "app", ".wahoo", "app/routes.go", metadataPath, legacyManifestPath, "go.mod"); err != nil {
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
	modulePath, err := projectModulePath(root)
	if err != nil {
		return err
	}

	var additions []Module
	for _, module := range modules {
		if contains(project.Modules, module) {
			continue
		}
		spec := moduleSpecs[module]
		for _, file := range spec.files {
			if err := rejectTargetSymlinks(root, file.target); err != nil {
				return err
			}
			if info, err := root.Lstat(file.target); err == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("managed path %s must not be a symbolic link", file.target)
				}
				return fmt.Errorf("module %q target already exists: %s", module, file.target)
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("inspect module %q target: %w", module, err)
			}
		}
		additions = append(additions, module)
	}
	if len(additions) == 0 {
		return nil
	}

	appName := ProjectName(filepath.Base(directory))
	routesChanged := false
	for _, module := range additions {
		spec := moduleSpecs[module]
		for _, file := range spec.files {
			data, err := fs.ReadFile(templates, file.template)
			if err != nil {
				return fmt.Errorf("read module %q template: %w", module, err)
			}
			if err := root.MkdirAll(filepath.ToSlash(filepath.Dir(file.target)), 0o755); err != nil {
				return fmt.Errorf("create module %q directory: %w", module, err)
			}
			content := renderTemplate(data, modulePath, appName, "", nil)
			if err := writeNewFile(root, file.target, content, 0o644); err != nil {
				return fmt.Errorf("write module %q: %w", module, err)
			}
		}
		if spec.registration != "" {
			routes = []byte(strings.Replace(string(routes), moduleMarker, spec.registration+moduleMarker, 1))
			routesChanged = true
		}
		project.Modules = append(project.Modules, module)
	}
	if routesChanged {
		if err := writeAtomic(root, "app/routes.go", routes, 0o644); err != nil {
			return fmt.Errorf("write application routes: %w", err)
		}
	}
	frameworkVersion, err := frameworkDependencyVersion(root)
	if err != nil {
		return err
	}
	project.FrameworkVersion = frameworkVersion
	return writeManifest(root, project)
}

func rejectManagedSymlinks(root *os.Root, paths ...string) error {
	for _, path := range paths {
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

func rejectTargetSymlinks(root *os.Root, target string) error {
	path := ""
	for _, part := range strings.Split(filepath.ToSlash(target), "/") {
		if path == "" {
			path = part
		} else {
			path += "/" + part
		}
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
	data, err := root.ReadFile(metadataPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return manifest{}, fmt.Errorf("read project metadata: %w", err)
		}
		data, err = root.ReadFile(legacyManifestPath)
		if errors.Is(err, fs.ErrNotExist) {
			return manifest{}, errors.New("not a Wahoo project: .wahoo/project.json is missing")
		}
		if err != nil {
			return manifest{}, fmt.Errorf("read legacy module manifest: %w", err)
		}
	}
	var project manifest
	if err := json.Unmarshal(data, &project); err != nil {
		return manifest{}, fmt.Errorf("decode project metadata: %w", err)
	}
	if project.FormatVersion != 0 && project.FormatVersion != metadataFormatVersion {
		return manifest{}, fmt.Errorf("unsupported project metadata format %d", project.FormatVersion)
	}
	modules, err := ParseModules(moduleStrings(project.Modules))
	if err != nil {
		return manifest{}, fmt.Errorf("validate module manifest: %w", err)
	}
	project.Modules = modules
	return project, nil
}

func writeManifest(root *os.Root, project manifest) error {
	project.FormatVersion = metadataFormatVersion
	if project.Modules == nil {
		project.Modules = []Module{}
	}
	if err := root.MkdirAll(".wahoo", 0o755); err != nil {
		return fmt.Errorf("create project metadata directory: %w", err)
	}
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project metadata: %w", err)
	}
	data = append(data, '\n')
	if err := writeAtomic(root, metadataPath, data, 0o644); err != nil {
		return fmt.Errorf("write project metadata: %w", err)
	}
	return nil
}

func writeNewFile(root *os.Root, path string, data []byte, perm fs.FileMode) error {
	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if written, err := file.Write(data); err != nil {
		file.Close()
		root.Remove(path)
		return err
	} else if written != len(data) {
		file.Close()
		root.Remove(path)
		return io.ErrShortWrite
	}
	if err := file.Close(); err != nil {
		root.Remove(path)
		return err
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

// Upgrade updates only the Wahoo dependency and project metadata. When apply
// is false it reports whether either file would change without writing files.
func Upgrade(directory, version string, apply bool) (UpgradeResult, error) {
	if !validFrameworkVersion(version) {
		return UpgradeResult{}, fmt.Errorf("invalid Wahoo version %q", version)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("inspect project directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return UpgradeResult{}, fmt.Errorf("project directory %s must not be a symbolic link", directory)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("open project root: %w", err)
	}
	defer root.Close()
	if err := rejectManagedSymlinks(root, ".wahoo", metadataPath, legacyManifestPath, "go.mod"); err != nil {
		return UpgradeResult{}, err
	}

	project, err := readManifest(root)
	if err != nil {
		return UpgradeResult{}, err
	}
	goMod, err := root.ReadFile("go.mod")
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("read Go module: %w", err)
	}
	current, updatedGoMod, err := replaceFrameworkRequirement(goMod, version)
	if err != nil {
		return UpgradeResult{}, err
	}
	projectChanged := project.FormatVersion != metadataFormatVersion || project.FrameworkVersion != version
	result := UpgradeResult{
		From:    current,
		To:      version,
		Changed: current != version || projectChanged,
	}
	if !apply || !result.Changed {
		return result, nil
	}
	if current != version {
		if err := writeAtomic(root, "go.mod", updatedGoMod, 0o644); err != nil {
			return UpgradeResult{}, fmt.Errorf("write Go module: %w", err)
		}
	}
	if projectChanged {
		project.FrameworkVersion = version
		if err := writeManifest(root, project); err != nil {
			return UpgradeResult{}, err
		}
	}
	return result, nil
}

// UpgradeResult describes a Wahoo dependency upgrade check or application.
type UpgradeResult struct {
	From    string
	To      string
	Changed bool
}

var frameworkVersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?(?:\+[0-9A-Za-z.-]+)?$`)

func validFrameworkVersion(version string) bool {
	return frameworkVersionPattern.MatchString(version)
}

func frameworkDependencyVersion(root *os.Root) (string, error) {
	data, err := root.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("read Go module: %w", err)
	}
	version, _, err := replaceFrameworkRequirement(data, "")
	return version, err
}

func projectModulePath(root *os.Root) (string, error) {
	data, err := root.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("read Go module: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", errors.New("Go module does not declare a module path")
}

func replaceFrameworkRequirement(data []byte, replacement string) (string, []byte, error) {
	lines := strings.SplitAfter(string(data), "\n")
	inRequireBlock := false
	found := 0
	current := ""
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && trimmed == ")" {
			inRequireBlock = false
			continue
		}

		candidate := ""
		if inRequireBlock {
			candidate = trimmed
		} else if strings.HasPrefix(trimmed, "require ") {
			candidate = strings.TrimSpace(strings.TrimPrefix(trimmed, "require "))
		}
		fields := strings.Fields(candidate)
		if len(fields) < 2 || fields[0] != "github.com/bjarneo/wahoo" {
			continue
		}
		found++
		if found > 1 {
			return "", nil, errors.New("Go module lists Wahoo more than once")
		}
		current = fields[1]
		if replacement == "" || replacement == current {
			continue
		}
		moduleOffset := strings.Index(line, "github.com/bjarneo/wahoo") + len("github.com/bjarneo/wahoo")
		versionOffset := strings.Index(line[moduleOffset:], current)
		if versionOffset < 0 {
			return "", nil, errors.New("locate Wahoo version in Go module")
		}
		versionOffset += moduleOffset
		lines[index] = line[:versionOffset] + replacement + line[versionOffset+len(current):]
	}
	if found == 0 {
		return "", nil, errors.New("Go module does not require github.com/bjarneo/wahoo")
	}
	return current, []byte(strings.Join(lines, "")), nil
}

func renderTemplate(data []byte, module, appName, replace string, modules []Module) []byte {
	content := strings.ReplaceAll(string(data), "__MODULE__", module)
	content = strings.ReplaceAll(content, "__APP_NAME__", appName)
	content = strings.ReplaceAll(content, "__REPLACE_LINE__", replace)
	content = strings.ReplaceAll(content, "__MODULES__", moduleList(modules))
	content = strings.ReplaceAll(content, "__FRAMEWORK_VERSION__", FrameworkVersion)
	return []byte(content)
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
