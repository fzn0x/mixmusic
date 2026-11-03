package settings

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestLiteralsNameTheSection keeps every literal of Settings in the
// repository naming the section of a field, as in
// settings.Settings{Playback: settings.Playback{Volume: 1}}. Go 1.27
// compiles settings.Settings{Volume: 1}, through the embedded Playback,
// but gopls and the other tools built with an older go/types call the
// field unknown, and editors show the file in errors.
func TestLiteralsNameTheSection(t *testing.T) {
	section := map[string]string{}
	sections := map[string]bool{}
	st := reflect.TypeFor[Settings]()
	for i := range st.NumField() {
		s := st.Field(i)
		if !s.Anonymous {
			t.Fatalf("Settings.%s isn't a section", s.Name)
		}
		sections[s.Name] = true
		for j := range s.Type.NumField() {
			section[s.Type.Field(j).Name] = s.Name
		}
	}
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "site", "testdata", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		inPackage := f.Name.Name == "settings"
		// isSettings tells whether a literal's type is Settings: its own
		// name in this package, settings.Settings elsewhere.
		isSettings := func(e ast.Expr) bool {
			switch e := e.(type) {
			case *ast.Ident:
				return inPackage && e.Name == "Settings"
			case *ast.SelectorExpr:
				x, ok := e.X.(*ast.Ident)
				return ok && x.Name == "settings" && e.Sel.Name == "Settings"
			}
			return false
		}
		check := func(lit *ast.CompositeLit) {
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || sections[key.Name] {
					continue
				}
				s := section[key.Name]
				t.Errorf("%s: a literal of Settings sets %s: write %s: %s{%s: …}",
					fset.Position(key.Pos()), key.Name, s, s, key.Name)
			}
		}
		// elided checks the elements of a map's or a slice's literal whose
		// type they leave out.
		elided := func(lit *ast.CompositeLit) {
			for _, el := range lit.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					el = kv.Value
				}
				if inner, ok := el.(*ast.CompositeLit); ok && inner.Type == nil {
					check(inner)
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			switch typ := lit.Type.(type) {
			case *ast.MapType:
				if isSettings(typ.Value) {
					elided(lit)
				}
			case *ast.ArrayType:
				if isSettings(typ.Elt) {
					elided(lit)
				}
			default:
				if isSettings(typ) {
					check(lit)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
