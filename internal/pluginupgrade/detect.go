package pluginupgrade

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

var (
	// ErrNotPlugin means the directory is not a Go module that requires finfocus-spec.
	ErrNotPlugin = errors.New("not a FinFocus plugin")
	// ErrTooOld means the plugin requires a finfocus-spec older than this package upgrades.
	ErrTooOld = errors.New("plugin is too old to upgrade")
	// ErrNewerThanCore means the plugin requires a newer finfocus-spec than this build knows.
	ErrNewerThanCore = errors.New("plugin is newer than this finfocus build")
	// ErrInvalidTarget means the requested target version cannot be planned.
	ErrInvalidTarget = errors.New("invalid target version")
)

// specVersionName is the identifier plugin init generates for the plugin's
// own record of its spec version.
const specVersionName = "SpecVersion"

// specVersionValue matches the literals worth rewriting: a version, with or
// without the v prefix. Anything else assigned to SpecVersion is not ours.
var specVersionValue = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

// SpecVersionDecl is a top-level SpecVersion string constant or variable.
type SpecVersionDecl struct {
	File  string `json:"file"`
	Value string `json:"value"`
}

// Project is what Detect found in a plugin directory. File paths are relative
// to Dir and use forward slashes.
type Project struct {
	Dir              string
	Version          string
	GoVersion        string
	ReplacePath      string
	ReplaceVersion   string
	SpecVersionDecls []SpecVersionDecl

	goMod   []byte
	sources map[string]*source
}

// source is a Go file with SpecVersion literals Apply may rewrite.
type source struct {
	content []byte
	decls   []span
}

// span is the byte range of a string literal, quotes included.
type span struct {
	start, end int
	value      string
}

// Detect reads the plugin in dir: its finfocus-spec requirement from go.mod
// and its SpecVersion declarations. A declaration counts only when it is a
// top-level version literal in a non-test file that imports finfocus-spec.
// It skips vendor, testdata, hidden directories, and nested modules.
func Detect(dir string) (*Project, error) {
	goModPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: no go.mod in %s", ErrNotPlugin, dir)
	}
	if err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}
	mf, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}

	p := &Project{Dir: dir, goMod: data, sources: map[string]*source{}}
	for _, r := range mf.Require {
		if r.Mod.Path == Module {
			p.Version = r.Mod.Version
		}
	}
	if p.Version == "" {
		return nil, fmt.Errorf("%w: go.mod in %s does not require %s", ErrNotPlugin, dir, Module)
	}
	if mf.Go != nil {
		p.GoVersion = mf.Go.Version
	}
	for _, r := range mf.Replace {
		if r.Old.Path == Module {
			p.ReplacePath = strings.TrimSpace(r.New.Path + " " + r.New.Version)
			p.ReplaceVersion = r.Old.Version
		}
	}

	if err = p.scanSources(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Project) scanSources() error {
	fset := token.NewFileSet()
	err := filepath.WalkDir(p.Dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != p.Dir && skipDir(path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(p.Dir, path)
		if err != nil {
			return err
		}
		return p.scanFile(fset, path, filepath.ToSlash(rel))
	})
	if err != nil {
		return fmt.Errorf("scanning plugin sources: %w", err)
	}
	sort.Slice(p.SpecVersionDecls, func(i, j int) bool {
		return p.SpecVersionDecls[i].File < p.SpecVersionDecls[j].File
	})
	return nil
}

func skipDir(path, name string) bool {
	if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

func (p *Project) scanFile(fset *token.FileSet, path, rel string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := parser.ParseFile(fset, path, content, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", rel, err)
	}
	if !importsSpec(f) {
		return nil
	}
	tf := fset.File(f.Pos())

	src := &source{content: content}
	for _, lit := range specVersionLiterals(f) {
		value, unquoteErr := strconv.Unquote(lit.Value)
		if unquoteErr != nil || !specVersionValue.MatchString(value) {
			continue
		}
		src.decls = append(src.decls, litSpan(tf, lit, value))
		p.SpecVersionDecls = append(p.SpecVersionDecls, SpecVersionDecl{File: rel, Value: value})
	}

	if len(src.decls) > 0 {
		p.sources[rel] = src
	}
	return nil
}

func litSpan(tf *token.File, lit *ast.BasicLit, value string) span {
	return span{start: tf.Offset(lit.Pos()), end: tf.Offset(lit.End()), value: value}
}

func importsSpec(f *ast.File) bool {
	for _, imp := range f.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err == nil && (importPath == Module || strings.HasPrefix(importPath, Module+"/")) {
			return true
		}
	}
	return false
}

// specVersionLiterals returns the string literals assigned to top-level
// constants or variables named SpecVersion.
func specVersionLiterals(f *ast.File) []*ast.BasicLit {
	var lits []*ast.BasicLit
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		for _, spec := range gen.Specs {
			if vs, isValue := spec.(*ast.ValueSpec); isValue {
				lits = append(lits, valueSpecLiterals(vs)...)
			}
		}
	}
	return lits
}

func valueSpecLiterals(vs *ast.ValueSpec) []*ast.BasicLit {
	var lits []*ast.BasicLit
	for i, name := range vs.Names {
		if name.Name != specVersionName || i >= len(vs.Values) {
			continue
		}
		if lit, isLit := vs.Values[i].(*ast.BasicLit); isLit && lit.Kind == token.STRING {
			lits = append(lits, lit)
		}
	}
	return lits
}
