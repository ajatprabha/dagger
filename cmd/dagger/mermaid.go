package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
)

// GenerateMermaid converts an AST expression tree of a DAG into Mermaid flowchart syntax.
// orientation can be "TD" (top-down) or "LR" (left-to-right).
func GenerateMermaid(root ast.Expr, orientation string) string {
	ori := "TD"
	if strings.ToUpper(strings.TrimSpace(orientation)) == "LR" {
		ori = "LR"
	}

	if root == nil {
		return fmt.Sprintf("flowchart %s\n    empty[\"Empty DAG\"]\n", ori)
	}

	g := &mermaidGenerator{
		orientation: ori,
	}

	res := g.build(root)
	_ = res // entry and exits used during build

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("flowchart %s\n", g.orientation))

	// Node definitions
	for _, nodeDef := range g.nodeDefs {
		sb.WriteString(nodeDef)
		sb.WriteString("\n")
	}

	if len(g.edges) > 0 {
		sb.WriteString("\n")
		for _, edge := range g.edges {
			sb.WriteString(edge)
			sb.WriteString("\n")
		}
	}

	// Visual styles
	if len(g.stepNodeIDs) > 0 || len(g.condNodeIDs) > 0 {
		sb.WriteString("\n")
		if len(g.stepNodeIDs) > 0 {
			sb.WriteString("    classDef step fill:#e0f2fe,stroke:#0284c7,stroke-width:2px,rx:6px,ry:6px;\n")
			sb.WriteString(fmt.Sprintf("    class %s step;\n", strings.Join(g.stepNodeIDs, ",")))
		}
		if len(g.condNodeIDs) > 0 {
			sb.WriteString("    classDef cond fill:#fef3c7,stroke:#d97706,stroke-width:2px;\n")
			sb.WriteString(fmt.Sprintf("    class %s cond;\n", strings.Join(g.condNodeIDs, ",")))
		}
	}

	return sb.String()
}

type mermaidGenerator struct {
	orientation string
	nodeSeq     int
	nodeDefs    []string
	edges       []string
	stepNodeIDs []string
	condNodeIDs []string
}

type flowExit struct {
	nodeID string
	label  string
	dotted bool
}

type flowResult struct {
	entry string
	exits []flowExit
}

func (g *mermaidGenerator) nextNodeID(prefix string) string {
	g.nodeSeq++
	return fmt.Sprintf("%s_%d", prefix, g.nodeSeq)
}

func (g *mermaidGenerator) newStepNode(label string) string {
	id := g.nextNodeID("step")
	safe := sanitize(label)
	// Rounded rectangle: id("label")
	g.nodeDefs = append(g.nodeDefs, fmt.Sprintf("    %s(\"%s\")", id, safe))
	g.stepNodeIDs = append(g.stepNodeIDs, id)
	return id
}

func (g *mermaidGenerator) newCondNode(label string) string {
	id := g.nextNodeID("cond")
	safe := sanitize(label)
	// Decision diamond: id{"label?"}
	g.nodeDefs = append(g.nodeDefs, fmt.Sprintf("    %s{\"%s\"}", id, safe))
	g.condNodeIDs = append(g.condNodeIDs, id)
	return id
}

func (g *mermaidGenerator) addEdge(from, to, label string, dotted bool) {
	if from == "" || to == "" {
		return
	}
	label = sanitize(label)
	arrow := "-->"
	if dotted {
		arrow = "-.->"
	}
	if label != "" {
		g.edges = append(g.edges, fmt.Sprintf("    %s %s|%s| %s", from, arrow, label, to))
	} else {
		g.edges = append(g.edges, fmt.Sprintf("    %s %s %s", from, arrow, to))
	}
}

func (g *mermaidGenerator) build(expr ast.Expr) flowResult {
	if expr == nil {
		id := g.newStepNode("nil")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	call, isCall := expr.(*ast.CallExpr)
	if !isCall {
		// Leaf node: identifier, selector, struct literal, func literal, etc.
		label := g.formatStepExpr(expr)
		id := g.newStepNode(label)
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	fun := unwrapGeneric(call.Fun)
	fnName := getCallFuncName(fun)

	switch fnName {
	case "Series":
		return g.buildSeries(call.Args)

	case "Continue":
		return g.buildContinue(call.Args)

	case "If":
		return g.buildIf(call.Args)

	case "IfNot":
		return g.buildIfNot(call.Args)

	case "IfElse":
		return g.buildIfElse(call.Args)

	case "Result":
		return g.buildResult(call.Args)

	case "NewStep", "NewResultErrStep":
		var label string
		if len(call.Args) > 0 {
			label = g.formatStepExpr(call.Args[0])
		} else {
			label = "step"
		}
		id := g.newStepNode(label)
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}

	case "New":
		if len(call.Args) > 0 {
			return g.build(call.Args[0])
		}
		id := g.newStepNode("DAG")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}

	default:
		// Unknown call: treat as a step
		label := g.formatStepExpr(expr)
		id := g.newStepNode(label)
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}
}

func (g *mermaidGenerator) buildSeries(args []ast.Expr) flowResult {
	if len(args) == 0 {
		id := g.newStepNode("Empty Series")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	res := g.build(args[0])
	for i := 1; i < len(args); i++ {
		next := g.build(args[i])
		for _, exit := range res.exits {
			g.addEdge(exit.nodeID, next.entry, exit.label, exit.dotted)
		}
		res.exits = next.exits
	}
	return res
}

func (g *mermaidGenerator) buildContinue(args []ast.Expr) flowResult {
	if len(args) == 0 {
		id := g.newStepNode("Empty Continue")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	res := g.build(args[0])
	for i := 1; i < len(args); i++ {
		next := g.build(args[i])
		for _, exit := range res.exits {
			label := exit.label
			if label == "" {
				label = "continue"
			}
			g.addEdge(exit.nodeID, next.entry, label, true)
		}
		res.exits = next.exits
	}
	return res
}

func (g *mermaidGenerator) buildIf(args []ast.Expr) flowResult {
	if len(args) < 2 {
		id := g.newStepNode("If")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	condLabel := g.formatCond(args[0])
	if !strings.HasSuffix(condLabel, "?") {
		condLabel += "?"
	}
	condID := g.newCondNode(condLabel)

	thenRes := g.build(args[1])
	g.addEdge(condID, thenRes.entry, "true", false)

	exits := append([]flowExit{}, thenRes.exits...)
	exits = append(exits, flowExit{nodeID: condID, label: "false"})

	return flowResult{entry: condID, exits: exits}
}

func (g *mermaidGenerator) buildIfNot(args []ast.Expr) flowResult {
	if len(args) < 2 {
		id := g.newStepNode("IfNot")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	condLabel := "not " + g.formatCond(args[0])
	if !strings.HasSuffix(condLabel, "?") {
		condLabel += "?"
	}
	condID := g.newCondNode(condLabel)

	thenRes := g.build(args[1])
	g.addEdge(condID, thenRes.entry, "false", false)

	exits := append([]flowExit{}, thenRes.exits...)
	exits = append(exits, flowExit{nodeID: condID, label: "true"})

	return flowResult{entry: condID, exits: exits}
}

func (g *mermaidGenerator) buildIfElse(args []ast.Expr) flowResult {
	if len(args) < 3 {
		id := g.newStepNode("IfElse")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	condLabel := g.formatCond(args[0])
	if !strings.HasSuffix(condLabel, "?") {
		condLabel += "?"
	}
	condID := g.newCondNode(condLabel)

	thenRes := g.build(args[1])
	elseRes := g.build(args[2])

	g.addEdge(condID, thenRes.entry, "true", false)
	g.addEdge(condID, elseRes.entry, "false", false)

	exits := append([]flowExit{}, thenRes.exits...)
	exits = append(exits, elseRes.exits...)

	return flowResult{entry: condID, exits: exits}
}

func (g *mermaidGenerator) buildResult(args []ast.Expr) flowResult {
	if len(args) == 0 {
		id := g.newStepNode("Result")
		return flowResult{entry: id, exits: []flowExit{{nodeID: id}}}
	}

	mainRes := g.build(args[0])
	var succRes *flowResult

	for _, opt := range args[1:] {
		optCall, ok := opt.(*ast.CallExpr)
		if !ok {
			continue
		}
		optFun := unwrapGeneric(optCall.Fun)
		optFnName := getCallFuncName(optFun)

		switch optFnName {
		case "OnSuccess":
			if len(optCall.Args) > 0 {
				sr := g.build(optCall.Args[0])
				succRes = &sr
				for _, exit := range mainRes.exits {
					g.addEdge(exit.nodeID, sr.entry, "success", false)
				}
			}

		case "OnError":
			if len(optCall.Args) > 0 {
				failRes := g.build(optCall.Args[0])
				for _, exit := range mainRes.exits {
					g.addEdge(exit.nodeID, failRes.entry, "error", false)
				}
			}

		case "Switch":
			for _, caseArg := range optCall.Args {
				caseCall, ok := caseArg.(*ast.CallExpr)
				if !ok {
					continue
				}
				cFun := unwrapGeneric(caseCall.Fun)
				cName := getCallFuncName(cFun)

				if cName == "Case" && len(caseCall.Args) >= 2 {
					condLabel := g.formatCond(caseCall.Args[0])
					stepRes := g.build(caseCall.Args[1])
					for _, exit := range mainRes.exits {
						g.addEdge(exit.nodeID, stepRes.entry, "case: "+condLabel, false)
					}
				} else if cName == "DefaultCase" && len(caseCall.Args) >= 1 {
					stepRes := g.build(caseCall.Args[0])
					for _, exit := range mainRes.exits {
						g.addEdge(exit.nodeID, stepRes.entry, "default", false)
					}
				}
			}
		}
	}

	if succRes != nil {
		return flowResult{entry: mainRes.entry, exits: succRes.exits}
	}
	return mainRes
}

func (g *mermaidGenerator) formatStepExpr(expr ast.Expr) string {
	if expr == nil {
		return "nil"
	}

	switch e := expr.(type) {
	case *ast.CallExpr:
		fun := unwrapGeneric(e.Fun)
		fnName := getCallFuncName(fun)
		if fnName == "NewStep" || fnName == "NewResultErrStep" {
			if len(e.Args) > 0 {
				return g.formatStepExpr(e.Args[0])
			}
			return "step"
		}
		return g.formatNodeString(expr)

	case *ast.Ident:
		return e.Name

	case *ast.SelectorExpr:
		return g.formatStepExpr(e.X) + "." + e.Sel.Name

	case *ast.FuncLit:
		var params []string
		if e.Type.Params != nil {
			for _, p := range e.Type.Params.List {
				for _, name := range p.Names {
					params = append(params, name.Name)
				}
			}
		}
		if len(params) > 0 {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func()"

	case *ast.UnaryExpr:
		return g.formatStepExpr(e.X)

	case *ast.CompositeLit:
		if e.Type != nil {
			return g.formatStepExpr(e.Type)
		}
		return "struct"

	case *ast.StarExpr:
		return g.formatStepExpr(e.X)

	default:
		return g.formatNodeString(expr)
	}
}

func (g *mermaidGenerator) formatCond(expr ast.Expr) string {
	if expr == nil {
		return "cond"
	}

	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name

	case *ast.SelectorExpr:
		return g.formatStepExpr(e.X) + "." + e.Sel.Name

	case *ast.FuncLit:
		// If single return statement in lambda, extract return expression
		if len(e.Body.List) == 1 {
			if ret, ok := e.Body.List[0].(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
				s := g.formatNodeString(ret.Results[0])
				if len(s) > 40 {
					return s[:37] + "..."
				}
				return s
			}
		}
		var params []string
		if e.Type.Params != nil {
			for _, p := range e.Type.Params.List {
				for _, name := range p.Names {
					params = append(params, name.Name)
				}
			}
		}
		if len(params) > 0 {
			return "cond(" + strings.Join(params, ", ") + ")"
		}
		return "cond()"

	default:
		s := g.formatNodeString(expr)
		if len(s) > 40 {
			return s[:37] + "..."
		}
		return s
	}
}

func (g *mermaidGenerator) formatNodeString(node ast.Node) string {
	var buf bytes.Buffer
	fset := token.NewFileSet()
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return fmt.Sprintf("%T", node)
	}
	s := strings.TrimSpace(buf.String())
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

func getCallFuncName(fun ast.Expr) string {
	fun = unwrapGeneric(fun)
	if sel, ok := fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	if id, ok := fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, `"`, "#quot;")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "|", "/")
	return strings.TrimSpace(s)
}
