package contexting

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// extractGoSymbols includes package declarations and receiver methods, while
// excluding local variables and anonymous functions.
func extractGoSymbols(path string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
	if err != nil {
		return nil, err
	}
	var symbols []string
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			symbols = append(symbols, declaration.Name.Name)
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					symbols = append(symbols, spec.Name.Name)
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.Name != "_" {
							symbols = append(symbols, name.Name)
						}
					}
				}
			}
		}
	}
	return dedupeStrings(symbols), nil
}
