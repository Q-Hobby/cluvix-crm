//go:build ignore

// check_i18n_keys: verify every sentinel key declared in pkg/errs/*.go
// has a translation in all locales (vi/en/jp/fil) under pkg/i18n/.
//
// Usage:
//
//	go run scripts/check_i18n_keys.go
//
// Exit code 0 = ok, 1 = missing keys.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	errsKeys, err := extractErrsKeys("pkg/errs")
	if err != nil {
		fail(err)
	}

	locales := []string{"vi", "en", "jp", "fil"}
	localeKeys := map[string]map[string]bool{}
	for _, l := range locales {
		keys, err := extractI18nKeys(filepath.Join("pkg/i18n", l+".go"))
		if err != nil {
			fail(err)
		}
		localeKeys[l] = keys
	}

	missing := map[string][]string{}
	for k := range errsKeys {
		for _, l := range locales {
			if !localeKeys[l][k] {
				missing[l] = append(missing[l], k)
			}
		}
	}

	if len(missing) == 0 {
		fmt.Printf("OK: %d errs key, all present in vi/en/jp/fil\n", len(errsKeys))
		return
	}

	fmt.Fprintln(os.Stderr, "Missing i18n translations:")
	for _, l := range locales {
		ks := missing[l]
		if len(ks) == 0 {
			continue
		}
		sort.Strings(ks)
		fmt.Fprintf(os.Stderr, "  %s.go:\n", l)
		for _, k := range ks {
			fmt.Fprintf(os.Stderr, "    %q\n", k)
		}
	}
	os.Exit(1)
}

// extractErrsKeys walks every .go file in dir, finds:
//   - errors.New("KEY") arguments
//   - errs.NewLocalizedError("KEY", ...) arguments (including its own package: NewLocalizedError(...))
func extractErrsKeys(dir string) (map[string]bool, error) {
	keys := map[string]bool{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			if !isCallTo(call.Fun, "errors", "New") && !isCallTo(call.Fun, "", "NewLocalizedError") {
				return true
			}
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				k := strings.Trim(lit.Value, "`\"")
				keys[k] = true
			}
			return true
		})
	}
	return keys, nil
}

// extractI18nKeys reads a locale file (e.g. pkg/i18n/vi.go) and returns the
// set of map keys defined in its top-level `var <locale> = map[string]string{...}`.
func extractI18nKeys(path string) (map[string]bool, error) {
	keys := map[string]bool{}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// Look for KeyValueExpr pairs with string key.
		for _, el := range cl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			lit, ok := kv.Key.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			keys[strings.Trim(lit.Value, "`\"")] = true
		}
		return true
	})
	return keys, nil
}

// isCallTo reports whether expr is a call of the form pkg.Name or (if pkg=="") Name.
func isCallTo(expr ast.Expr, pkg, name string) bool {
	switch fn := expr.(type) {
	case *ast.SelectorExpr:
		id, ok := fn.X.(*ast.Ident)
		if !ok {
			return false
		}
		return id.Name == pkg && fn.Sel.Name == name
	case *ast.Ident:
		return pkg == "" && fn.Name == name
	}
	return false
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(2)
}
