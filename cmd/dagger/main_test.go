package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASTDiscovery_ExampleTest(t *testing.T) {
	// Find project root by looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	rootDir := dir
	for {
		if _, err := os.Stat(filepath.Join(rootDir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(rootDir)
		if parent == rootDir {
			t.Fatalf("could not find project root (containing go.mod)")
		}
		rootDir = parent
	}

	dags, err := DiscoverDAGs(rootDir)
	if err != nil {
		t.Fatalf("DiscoverDAGs failed: %v", err)
	}

	if len(dags) < 8 {
		t.Fatalf("expected at least 8 DAGs, found %d", len(dags))
	}

	expectedExamples := []string{
		"ExampleNew",
		"ExampleIf",
		"ExampleIfNot",
		"ExampleIfElse",
		"ExampleResult",
		"ExampleSeries",
		"ExampleContinue",
		"ExampleResult_switch",
	}

	foundMap := make(map[string]*DiscoveredDAG)
	for _, dag := range dags {
		foundMap[dag.Name] = dag
	}

	for _, name := range expectedExamples {
		dag, found := foundMap[name]
		if !found {
			t.Errorf("expected to discover DAG with name %q, but not found", name)
			continue
		}

		if dag.Package != "dagger_test" {
			t.Errorf("expected package 'dagger_test' for %s, got %q", name, dag.Package)
		}

		if !strings.HasSuffix(dag.File, "example_test.go") {
			t.Errorf("expected file ending in example_test.go for %s, got %q", name, dag.File)
		}

		if dag.Line <= 0 {
			t.Errorf("expected valid line number for %s, got %d", name, dag.Line)
		}

		if dag.RootExpr == nil {
			t.Errorf("expected non-nil RootExpr for %s", name)
		}
	}
}

func TestASTDiscovery_VariableResolution(t *testing.T) {
	tempDir := t.TempDir()
	src := `package testpkg

import "github.com/ajatprabha/dagger"

func BuildMyPipeline() {
	step1 := dagger.NewStep(stepOneFunc)
	step2 := dagger.NewStep(stepTwoFunc)
	pipeline := dagger.Series(step1, step2)
	dag, err := dagger.New(pipeline)
	_ = dag
	_ = err
}
`
	testFile := filepath.Join(tempDir, "pipeline.go")
	if err := os.WriteFile(testFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	dags, err := DiscoverDAGs(tempDir)
	if err != nil {
		t.Fatalf("DiscoverDAGs failed: %v", err)
	}

	if len(dags) != 1 {
		t.Fatalf("expected 1 DAG, found %d", len(dags))
	}

	dag := dags[0]
	if dag.Name != "BuildMyPipeline" {
		t.Errorf("expected name BuildMyPipeline, got %q", dag.Name)
	}

	// Verify mermaid output contains resolved steps
	mermaid := GenerateMermaid(dag.RootExpr, "TD")
	if !strings.Contains(mermaid, "stepOneFunc") {
		t.Errorf("expected resolved stepOneFunc in mermaid output, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "stepTwoFunc") {
		t.Errorf("expected resolved stepTwoFunc in mermaid output, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "-->") {
		t.Errorf("expected sequential edge in mermaid output, got:\n%s", mermaid)
	}
}

func TestASTDiscovery_ReturnedDAGFunction(t *testing.T) {
	tempDir := t.TempDir()
	src := `package testpkg

import "github.com/ajatprabha/dagger"

func MakeWorkflow() dagger.Step[any] {
	return dagger.Series(
		dagger.NewStep(firstStep),
		dagger.NewStep(secondStep),
	)
}
`
	testFile := filepath.Join(tempDir, "workflow.go")
	if err := os.WriteFile(testFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	dags, err := DiscoverDAGs(tempDir)
	if err != nil {
		t.Fatalf("DiscoverDAGs failed: %v", err)
	}

	if len(dags) != 1 {
		t.Fatalf("expected 1 DAG from returned function, found %d", len(dags))
	}

	if dags[0].Name != "MakeWorkflow" {
		t.Errorf("expected MakeWorkflow, got %q", dags[0].Name)
	}
}

func parseDAGRoot(t *testing.T, src string) ast.Expr {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	var root ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun := unwrapGeneric(call.Fun)
		if sel, ok := fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "New" {
			if len(call.Args) > 0 {
				root = call.Args[0]
				return false
			}
		}
		return true
	})

	if root == nil {
		t.Fatalf("failed to find dagger.New call in snippet")
	}
	return root
}

func TestMermaidGeneration_Series(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.Series(
		dagger.NewStep(step1),
		dagger.NewStep(step2),
		dagger.NewStep(step3),
	),
)
`
	root := parseDAGRoot(t, src)

	mermaidTD := GenerateMermaid(root, "TD")
	if !strings.HasPrefix(mermaidTD, "flowchart TD\n") {
		t.Errorf("expected 'flowchart TD' header, got:\n%s", mermaidTD)
	}
	if !strings.Contains(mermaidTD, "step1") || !strings.Contains(mermaidTD, "step2") || !strings.Contains(mermaidTD, "step3") {
		t.Errorf("missing step labels in:\n%s", mermaidTD)
	}
	if !strings.Contains(mermaidTD, "-->") {
		t.Errorf("missing sequential edges in:\n%s", mermaidTD)
	}

	mermaidLR := GenerateMermaid(root, "LR")
	if !strings.HasPrefix(mermaidLR, "flowchart LR\n") {
		t.Errorf("expected 'flowchart LR' header, got:\n%s", mermaidLR)
	}
}

func TestMermaidGeneration_Continue(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.Continue(
		dagger.NewStep(validate),
		dagger.NewStep(process),
	),
)
`
	root := parseDAGRoot(t, src)

	mermaid := GenerateMermaid(root, "TD")
	if !strings.Contains(mermaid, "-.->|continue|") {
		t.Errorf("expected dotted continue edge, got:\n%s", mermaid)
	}
}

func TestMermaidGeneration_If(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.If(
		hasPermission,
		dagger.NewStep(grantAccess),
	),
)
`
	root := parseDAGRoot(t, src)

	mermaid := GenerateMermaid(root, "TD")
	if !strings.Contains(mermaid, "{\"hasPermission?\"}") {
		t.Errorf("expected condition diamond with hasPermission?, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "-->|true|") {
		t.Errorf("expected true edge from condition diamond, got:\n%s", mermaid)
	}
}

func TestMermaidGeneration_IfNot(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.IfNot(
		isBlocked,
		dagger.NewStep(proceed),
	),
)
`
	root := parseDAGRoot(t, src)

	mermaid := GenerateMermaid(root, "TD")
	if !strings.Contains(mermaid, "{\"not isBlocked?\"}") {
		t.Errorf("expected condition diamond with not isBlocked?, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "-->|false|") {
		t.Errorf("expected false edge from condition diamond, got:\n%s", mermaid)
	}
}

func TestMermaidGeneration_IfElse(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.IfElse(
		isValid,
		dagger.NewStep(thenStep),
		dagger.NewStep(elseStep),
	),
)
`
	root := parseDAGRoot(t, src)

	mermaid := GenerateMermaid(root, "TD")
	if !strings.Contains(mermaid, "{\"isValid?\"}") {
		t.Errorf("expected condition diamond with isValid?, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "-->|true|") {
		t.Errorf("expected true edge in:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "-->|false|") {
		t.Errorf("expected false edge in:\n%s", mermaid)
	}
}

func TestMermaidGeneration_Result(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.Result(
		dagger.NewStep(mainAction),
		dagger.OnSuccess(dagger.NewStep(successAction)),
		dagger.OnError(dagger.NewStep(errorAction)),
	),
)
`
	root := parseDAGRoot(t, src)

	mermaid := GenerateMermaid(root, "TD")
	if !strings.Contains(mermaid, "-->|success|") {
		t.Errorf("expected success edge, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "-->|error|") {
		t.Errorf("expected error edge, got:\n%s", mermaid)
	}
}

func TestMermaidGeneration_ResultSwitch(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.Result(
		dagger.NewStep(callService),
		dagger.OnSuccess(dagger.NewStep(logSuccess)),
		dagger.Switch[myState](
			dagger.Case(isTimeout, dagger.NewStep(retry)),
			dagger.DefaultCase(dagger.NewStep(abort)),
		),
	),
)
`
	root := parseDAGRoot(t, src)

	mermaid := GenerateMermaid(root, "TD")
	if !strings.Contains(mermaid, "-->|case: isTimeout|") {
		t.Errorf("expected case edge with isTimeout, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "-->|default|") {
		t.Errorf("expected default edge, got:\n%s", mermaid)
	}
}

func TestMermaidGeneration_DirectIdentifiersAndSanitization(t *testing.T) {
	src := `package main
import "github.com/ajatprabha/dagger"
var _ = dagger.New(
	dagger.Series(
		r.validateResourceQuotas,
		allocateIP,
		&customStructStep{},
	),
)
`
	root := parseDAGRoot(t, src)

	mermaid := GenerateMermaid(root, "TD")
	if !strings.Contains(mermaid, "r.validateResourceQuotas") {
		t.Errorf("expected r.validateResourceQuotas in mermaid output:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "allocateIP") {
		t.Errorf("expected allocateIP in mermaid output:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "customStructStep") {
		t.Errorf("expected customStructStep in mermaid output:\n%s", mermaid)
	}
}

func TestCLI_Flags(t *testing.T) {
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(repoDir, "go.mod")); err == nil {
			break
		}
		repoDir = filepath.Dir(repoDir)
	}

	// 1. Test -list flag
	{
		var stdout, stderr bytes.Buffer
		err := run([]string{"-dir", repoDir, "-list"}, &stdout, &stderr)
		if err != nil {
			t.Fatalf("run -list failed: %v\nstderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "Discovered") || !strings.Contains(out, "ExampleSeries") {
			t.Errorf("unexpected output from -list:\n%s", out)
		}
	}

	// 2. Test -stdout flag with -select
	{
		var stdout, stderr bytes.Buffer
		err := run([]string{"-dir", repoDir, "-stdout", "-select", "ExampleSeries"}, &stdout, &stderr)
		if err != nil {
			t.Fatalf("run -stdout -select failed: %v\nstderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "flowchart TD") || !strings.Contains(out, "validateResource") {
			t.Errorf("unexpected output from -stdout -select:\n%s", out)
		}
	}

	// 3. Test -stdout flag with -select by index
	{
		var stdout, stderr bytes.Buffer
		err := run([]string{"-dir", repoDir, "-stdout", "-select", "0"}, &stdout, &stderr)
		if err != nil {
			t.Fatalf("run -stdout -select 0 failed: %v\nstderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "flowchart TD") {
			t.Errorf("unexpected output from -stdout -select 0:\n%s", out)
		}
	}

	// 4. Test -stdout with -orientation LR
	{
		var stdout, stderr bytes.Buffer
		err := run([]string{"-dir", repoDir, "-stdout", "-select", "ExampleSeries", "-orientation", "LR"}, &stdout, &stderr)
		if err != nil {
			t.Fatalf("run -stdout -orientation LR failed: %v", err)
		}
		out := stdout.String()
		if !strings.Contains(out, "flowchart LR") {
			t.Errorf("expected 'flowchart LR', got:\n%s", out)
		}
	}

	// 5. Test -select with nonexistent name
	{
		var stdout, stderr bytes.Buffer
		err := run([]string{"-dir", repoDir, "-stdout", "-select", "NonExistentDAGXYZ"}, &stdout, &stderr)
		if err == nil {
			t.Errorf("expected error for nonexistent DAG selection, got nil")
		}
	}
}

func TestServer_HTTPHandler(t *testing.T) {
	dags := []*DiscoveredDAG{
		{
			ID:       "dag-0",
			Name:     "TestWorkflow",
			Package:  "testpkg",
			File:     "test.go",
			Line:     42,
			RootExpr: nil,
		},
	}

	handler := createMux(dags, "TD")
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// 1. Test GET / dashboard
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from GET /, got %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	bodyStr := buf.String()

	// Verify cdnjs mermaid link is present
	const expectedCDN = "https://cdnjs.cloudflare.com/ajax/libs/mermaid/10.9.0/mermaid.min.js"
	if !strings.Contains(bodyStr, expectedCDN) {
		t.Errorf("dashboard HTML missing cdnjs mermaid link %q", expectedCDN)
	}

	// Verify dropdown and buttons are present
	if !strings.Contains(bodyStr, `id="dagSelector"`) {
		t.Errorf("dashboard HTML missing dagSelector element")
	}
	if !strings.Contains(bodyStr, "Copy Mermaid") {
		t.Errorf("dashboard HTML missing Copy Mermaid button")
	}
	if !strings.Contains(bodyStr, "Raw Mermaid") {
		t.Errorf("dashboard HTML missing Raw Mermaid button")
	}

	// Verify pan/zoom controls and stage are present
	if !strings.Contains(bodyStr, `id="canvasContainer"`) {
		t.Errorf("dashboard HTML missing canvasContainer element")
	}
	if !strings.Contains(bodyStr, `id="diagram-stage"`) {
		t.Errorf("dashboard HTML missing diagram-stage element")
	}
	if !strings.Contains(bodyStr, `class="canvas-controls"`) {
		t.Errorf("dashboard HTML missing canvas-controls toolbar")
	}
	if !strings.Contains(bodyStr, `id="zoomLevel"`) {
		t.Errorf("dashboard HTML missing zoomLevel indicator")
	}

	// 2. Test GET /api/dags
	respAPI, err := http.Get(ts.URL + "/api/dags")
	if err != nil {
		t.Fatalf("GET /api/dags failed: %v", err)
	}
	defer respAPI.Body.Close()

	if respAPI.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from /api/dags, got %d", respAPI.StatusCode)
	}

	var models []DAGViewModel
	if err := json.NewDecoder(respAPI.Body).Decode(&models); err != nil {
		t.Fatalf("failed to decode JSON from /api/dags: %v", err)
	}

	if len(models) != 1 || models[0].Name != "TestWorkflow" {
		t.Errorf("unexpected models from /api/dags: %+v", models)
	}

	// 3. Test GET /api/mermaid
	respMermaid, err := http.Get(ts.URL + "/api/mermaid?id=dag-0&orientation=LR")
	if err != nil {
		t.Fatalf("GET /api/mermaid failed: %v", err)
	}
	defer respMermaid.Body.Close()

	if respMermaid.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from /api/mermaid, got %d", respMermaid.StatusCode)
	}

	var mBuf bytes.Buffer
	_, _ = mBuf.ReadFrom(respMermaid.Body)
	mStr := mBuf.String()
	if !strings.Contains(mStr, "flowchart LR") {
		t.Errorf("expected flowchart LR from /api/mermaid?orientation=LR, got:\n%s", mStr)
	}
}
