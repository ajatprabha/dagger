package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// DiscoveredDAG represents a DAG found via AST inspection.
type DiscoveredDAG struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Package  string   `json:"package"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	RootExpr ast.Expr `json:"-"`
}

// DiscoverDAGs recursively scans .go files in rootDir and discovers DAGs.
// It skips .git, vendor, and hidden directories.
func DiscoverDAGs(rootDir string) ([]*DiscoveredDAG, error) {
	var dags []*DiscoveredDAG
	fset := token.NewFileSet()

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			if path != rootDir && (name == ".git" || name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		fileDAGs, parseErr := parseFileForDAGs(fset, rootDir, path)
		if parseErr != nil {
			// Skip unparseable files rather than failing entire scan
			return nil
		}

		dags = append(dags, fileDAGs...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed scanning directory %s: %w", rootDir, err)
	}

	// Assign sequential IDs
	for i, dag := range dags {
		dag.ID = fmt.Sprintf("dag-%d", i)
	}

	return dags, nil
}

type fileContext struct {
	fset        *token.FileSet
	relPath     string
	pkgName     string
	isDaggerPkg bool
	daggerAlias string
	fileScope   map[string]ast.Expr
}

func parseFileForDAGs(fset *token.FileSet, rootDir, absPath string) ([]*DiscoveredDAG, error) {
	node, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(rootDir, absPath)
	if err != nil {
		relPath = absPath
	}

	fc := &fileContext{
		fset:      fset,
		relPath:   relPath,
		pkgName:   node.Name.Name,
		fileScope: make(map[string]ast.Expr),
	}

	if fc.pkgName == "dagger" {
		fc.isDaggerPkg = true
	}

	// Inspect imports to detect package alias for dagger
	for _, imp := range node.Imports {
		pathVal := strings.Trim(imp.Path.Value, `"`)
		if pathVal == "github.com/ajatprabha/dagger" || strings.HasSuffix(pathVal, "/dagger") {
			if imp.Name != nil {
				fc.daggerAlias = imp.Name.Name
			} else {
				fc.daggerAlias = "dagger"
			}
		}
	}

	// Collect file-level variable declarations into fileScope
	for _, decl := range node.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
			for _, spec := range gen.Specs {
				if vspec, ok := spec.(*ast.ValueSpec); ok {
					if len(vspec.Values) == len(vspec.Names) {
						for i, name := range vspec.Names {
							fc.fileScope[name.Name] = vspec.Values[i]
						}
					}
				}
			}
		}
	}

	var dags []*DiscoveredDAG
	visitedPositions := make(map[token.Pos]bool)

	// Helper to add a DAG avoiding duplicate positions
	addDAG := func(pos token.Pos, name string, rootExpr ast.Expr) {
		if visitedPositions[pos] {
			return
		}
		visitedPositions[pos] = true

		line := fset.Position(pos).Line
		dags = append(dags, &DiscoveredDAG{
			Name:     name,
			Package:  fc.pkgName,
			File:     fc.relPath,
			Line:     line,
			RootExpr: rootExpr,
		})
	}

	// Check top-level package variables (e.g. var dag, err = dagger.New(...))
	for _, decl := range node.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
			for _, spec := range gen.Specs {
				if vspec, ok := spec.(*ast.ValueSpec); ok {
					for i, val := range vspec.Values {
						if fc.isDaggerNewCall(val) {
							call := val.(*ast.CallExpr)
							var varName string
							if i < len(vspec.Names) {
								varName = vspec.Names[i].Name
							}
							var root ast.Expr
							if len(call.Args) > 0 {
								root = fc.resolveExpr(call.Args[0], fc.fileScope, make(map[string]bool))
							}
							name := determineDAGName("", varName, root)
							addDAG(val.Pos(), name, root)
						}
					}
				}
			}
		}
	}

	// Inspect functions
	for _, decl := range node.Decls {
		fnDecl, ok := decl.(*ast.FuncDecl)
		if !ok || fnDecl.Body == nil {
			continue
		}

		funcName := fnDecl.Name.Name
		scope := copyScope(fc.fileScope)
		funcFoundDAG := false

		// Walk statements and blocks within the function
		walkBlock(fnDecl.Body, funcName, scope, fc, func(pos token.Pos, name string, root ast.Expr) {
			funcFoundDAG = true
			addDAG(pos, name, root)
		})

		// Also check if the function itself is a DAG builder/constructor returning a DAG
		if !funcFoundDAG {
			if root := fc.detectReturnedDAG(fnDecl.Body, scope); root != nil {
				resolvedRoot := fc.resolveExpr(root, scope, make(map[string]bool))
				name := determineDAGName(funcName, "", resolvedRoot)
				addDAG(fnDecl.Pos(), name, resolvedRoot)
			}
		}
	}

	return dags, nil
}

func walkBlock(body *ast.BlockStmt, parentName string, scope map[string]ast.Expr, fc *fileContext, onDAG func(pos token.Pos, name string, root ast.Expr)) {
	if body == nil {
		return
	}

	for _, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			// Check for assignments like:
			// pipeline := dagger.Series(...)
			// dag, err := dagger.New(pipeline)
			for i, rhs := range s.Rhs {
				if fc.isDaggerNewCall(rhs) {
					call := rhs.(*ast.CallExpr)
					var varName string
					if i < len(s.Lhs) {
						if id, ok := s.Lhs[i].(*ast.Ident); ok {
							varName = id.Name
						}
					}
					var root ast.Expr
					if len(call.Args) > 0 {
						root = fc.resolveExpr(call.Args[0], scope, make(map[string]bool))
					}
					name := determineDAGName(parentName, varName, root)
					onDAG(s.Pos(), name, root)
				}
			}

			// Record assignments in local scope
			if len(s.Lhs) == len(s.Rhs) {
				for i, lhs := range s.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						scope[id.Name] = s.Rhs[i]
					}
				}
			}

		case *ast.DeclStmt:
			if gen, ok := s.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				for _, spec := range gen.Specs {
					if vspec, ok := spec.(*ast.ValueSpec); ok {
						for i, val := range vspec.Values {
							if fc.isDaggerNewCall(val) {
								call := val.(*ast.CallExpr)
								var varName string
								if i < len(vspec.Names) {
									varName = vspec.Names[i].Name
								}
								var root ast.Expr
								if len(call.Args) > 0 {
									root = fc.resolveExpr(call.Args[0], scope, make(map[string]bool))
								}
								name := determineDAGName(parentName, varName, root)
								onDAG(val.Pos(), name, root)
							}
							if i < len(vspec.Names) {
								scope[vspec.Names[i].Name] = val
							}
						}
					}
				}
			}

		case *ast.ReturnStmt:
			for _, res := range s.Results {
				if fc.isDaggerNewCall(res) {
					call := res.(*ast.CallExpr)
					var root ast.Expr
					if len(call.Args) > 0 {
						root = fc.resolveExpr(call.Args[0], scope, make(map[string]bool))
					}
					name := determineDAGName(parentName, "", root)
					onDAG(s.Pos(), name, root)
				}
			}

		case *ast.ExprStmt:
			if fc.isDaggerNewCall(s.X) {
				call := s.X.(*ast.CallExpr)
				var root ast.Expr
				if len(call.Args) > 0 {
					root = fc.resolveExpr(call.Args[0], scope, make(map[string]bool))
				}
				name := determineDAGName(parentName, "", root)
				onDAG(s.Pos(), name, root)
			} else if call, ok := s.X.(*ast.CallExpr); ok {
				// Detect t.Run("SubTest", func(t *testing.T) { ... })
				checkSubtestCall(call, parentName, scope, fc, onDAG)
			}

		case *ast.IfStmt:
			if s.Init != nil {
				walkBlock(&ast.BlockStmt{List: []ast.Stmt{s.Init}}, parentName, scope, fc, onDAG)
			}
			walkBlock(s.Body, parentName, copyScope(scope), fc, onDAG)
			if s.Else != nil {
				if elseBlock, ok := s.Else.(*ast.BlockStmt); ok {
					walkBlock(elseBlock, parentName, copyScope(scope), fc, onDAG)
				}
			}

		case *ast.ForStmt:
			walkBlock(s.Body, parentName, copyScope(scope), fc, onDAG)

		case *ast.RangeStmt:
			walkBlock(s.Body, parentName, copyScope(scope), fc, onDAG)

		case *ast.BlockStmt:
			walkBlock(s, parentName, copyScope(scope), fc, onDAG)
		}
	}
}

func checkSubtestCall(call *ast.CallExpr, parentName string, scope map[string]ast.Expr, fc *fileContext, onDAG func(pos token.Pos, name string, root ast.Expr)) {
	// Look for obj.Run("subTestName", func(t *testing.T) { ... })
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Run" {
		if len(call.Args) >= 2 {
			subName := ""
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				subName = strings.Trim(lit.Value, `"`)
			}
			if fnLit, ok := call.Args[1].(*ast.FuncLit); ok && fnLit.Body != nil {
				fullName := parentName
				if subName != "" {
					fullName = parentName + "/" + subName
				}
				walkBlock(fnLit.Body, fullName, copyScope(scope), fc, onDAG)
			}
		}
	}
}

// detectReturnedDAG checks if a function returns a constructed DAG (e.g. dagger.Series, etc.)
func (fc *fileContext) detectReturnedDAG(body *ast.BlockStmt, scope map[string]ast.Expr) ast.Expr {
	if body == nil {
		return nil
	}

	var candidate ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, res := range ret.Results {
			resolved := fc.resolveExpr(res, scope, make(map[string]bool))
			if fc.isDAGStepConstructor(resolved) {
				candidate = resolved
				return false
			}
		}
		return true
	})

	return candidate
}

func (fc *fileContext) isDaggerNewCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	_, matched := fc.isDaggerFunc(call.Fun, "New")
	return matched
}

func (fc *fileContext) isDAGStepConstructor(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	_, matched := fc.isDaggerFunc(call.Fun, "Series", "Continue", "If", "IfNot", "IfElse", "Result")
	return matched
}

// isDaggerFunc checks if fun refers to dagger.<targetFunc> or unqualified <targetFunc> in package dagger
func (fc *fileContext) isDaggerFunc(fun ast.Expr, targetFuncs ...string) (string, bool) {
	fun = unwrapGeneric(fun)
	if sel, ok := fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			if id.Name == fc.daggerAlias || (fc.daggerAlias == "" && id.Name == "dagger") {
				for _, fn := range targetFuncs {
					if sel.Sel.Name == fn {
						return fn, true
					}
				}
			}
		}
	} else if id, ok := fun.(*ast.Ident); ok {
		if fc.isDaggerPkg || fc.daggerAlias == "." {
			for _, fn := range targetFuncs {
				if id.Name == fn {
					return fn, true
				}
			}
		}
	}
	return "", false
}

func unwrapGeneric(expr ast.Expr) ast.Expr {
	switch e := expr.(type) {
	case *ast.IndexExpr:
		return unwrapGeneric(e.X)
	case *ast.IndexListExpr:
		return unwrapGeneric(e.X)
	case *ast.ParenExpr:
		return unwrapGeneric(e.X)
	default:
		return expr
	}
}

// resolveExpr recursively resolves variable identifiers to their assigned expressions within scope
func (fc *fileContext) resolveExpr(expr ast.Expr, scope map[string]ast.Expr, visited map[string]bool) ast.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if visited[e.Name] {
			return e
		}
		val, ok := scope[e.Name]
		if !ok {
			val, ok = fc.fileScope[e.Name]
		}
		if !ok {
			return e
		}

		// Only resolve if val is a DAG step, constructor call, struct, or another identifier
		if fc.shouldResolve(val) {
			visited[e.Name] = true
			resolved := fc.resolveExpr(val, scope, visited)
			visited[e.Name] = false
			return resolved
		}
		return e

	case *ast.CallExpr:
		newCall := *e
		newArgs := make([]ast.Expr, len(e.Args))
		for i, arg := range e.Args {
			newArgs[i] = fc.resolveExpr(arg, scope, visited)
		}
		newCall.Args = newArgs
		return &newCall

	case *ast.ParenExpr:
		newP := *e
		newP.X = fc.resolveExpr(e.X, scope, visited)
		return &newP

	case *ast.UnaryExpr:
		newU := *e
		newU.X = fc.resolveExpr(e.X, scope, visited)
		return &newU

	default:
		return expr
	}
}

func (fc *fileContext) shouldResolve(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.CallExpr:
		// Resolve any dagger step constructor, NewStep, or option calls
		_, ok := fc.isDaggerFunc(e.Fun,
			"Series", "Continue", "If", "IfNot", "IfElse", "Result",
			"OnSuccess", "OnError", "Switch", "Case", "DefaultCase",
			"NewStep", "NewResultErrStep", "New",
		)
		return ok
	case *ast.Ident:
		return true
	case *ast.UnaryExpr, *ast.CompositeLit:
		return true
	default:
		return false
	}
}

func copyScope(scope map[string]ast.Expr) map[string]ast.Expr {
	c := make(map[string]ast.Expr, len(scope))
	for k, v := range scope {
		c[k] = v
	}
	return c
}

func determineDAGName(fnName, varName string, rootExpr ast.Expr) string {
	if fnName != "" {
		if varName != "" && varName != "dag" && varName != "d" && varName != "_" {
			return fmt.Sprintf("%s (%s)", fnName, varName)
		}
		return fnName
	}
	if varName != "" && varName != "dag" && varName != "d" && varName != "_" {
		return varName
	}
	if rootExpr != nil {
		return getStepTypeName(rootExpr)
	}
	return "DAG"
}

func getStepTypeName(expr ast.Expr) string {
	if expr == nil {
		return "DAG"
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		fun := unwrapGeneric(call.Fun)
		if sel, ok := fun.(*ast.SelectorExpr); ok {
			return sel.Sel.Name
		}
		if id, ok := fun.(*ast.Ident); ok {
			return id.Name
		}
	}
	return "DAG"
}
