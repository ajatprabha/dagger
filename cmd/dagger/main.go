package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagger", flag.ContinueOnError)
	fs.SetOutput(stderr)

	flagDir := fs.String("dir", ".", "root directory to scan for DAGs")
	flagList := fs.Bool("list", false, "list all discovered DAGs to stdout and exit")
	flagSelect := fs.String("select", "", "select a specific DAG by index, name, or file:line substring")
	flagStdout := fs.Bool("stdout", false, "output the generated Mermaid template string to stdout and exit")
	flagServe := fs.String("serve", ":8080", "address to serve HTTP dashboard on")
	flagOrientation := fs.String("orientation", "TD", "flowchart orientation (TD or LR)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	dags, err := DiscoverDAGs(*flagDir)
	if err != nil {
		return fmt.Errorf("error discovering DAGs: %w", err)
	}

	if *flagList {
		return printDAGList(stdout, dags)
	}

	if *flagStdout {
		selected, err := selectDAG(dags, *flagSelect)
		if err != nil {
			return err
		}
		mermaid := GenerateMermaid(selected.RootExpr, *flagOrientation)
		_, err = fmt.Fprint(stdout, mermaid)
		return err
	}

	// Serve HTTP dashboard by default or when -serve is passed
	if len(dags) == 0 {
		if _, err := fmt.Fprintf(stderr, "Warning: no DAGs discovered in %s\n", *flagDir); err != nil {
			return err
		}
	}
	return startServer(*flagServe, dags, *flagOrientation)
}

func printDAGList(w io.Writer, dags []*DiscoveredDAG) error {
	if len(dags) == 0 {
		_, err := fmt.Fprintln(w, "No DAGs discovered.")
		return err
	}
	if _, err := fmt.Fprintf(w, "Discovered %d DAG(s):\n", len(dags)); err != nil {
		return err
	}
	for i, d := range dags {
		if _, err := fmt.Fprintf(w, "[%d] %s (%s) - %s:%d (ID: %s)\n", i, d.Name, d.Package, d.File, d.Line, d.ID); err != nil {
			return err
		}
	}
	return nil
}

func selectDAG(dags []*DiscoveredDAG, pattern string) (*DiscoveredDAG, error) {
	if len(dags) == 0 {
		return nil, fmt.Errorf("no DAGs discovered")
	}
	if pattern == "" {
		return dags[0], nil
	}

	// 1. Try parsing as an integer index
	if idx, err := strconv.Atoi(pattern); err == nil {
		if idx >= 0 && idx < len(dags) {
			return dags[idx], nil
		}
		return nil, fmt.Errorf("index %d out of range [0, %d)", idx, len(dags))
	}

	// 2. Check exact ID match
	for _, d := range dags {
		if strings.EqualFold(d.ID, pattern) {
			return d, nil
		}
	}

	// 3. Check exact Name match
	for _, d := range dags {
		if strings.EqualFold(d.Name, pattern) {
			return d, nil
		}
	}

	// 4. Check Name substring
	patternLower := strings.ToLower(pattern)
	for _, d := range dags {
		if strings.Contains(strings.ToLower(d.Name), patternLower) {
			return d, nil
		}
	}

	// 5. Check File:Line or File substring
	for _, d := range dags {
		loc := fmt.Sprintf("%s:%d", d.File, d.Line)
		if strings.Contains(loc, pattern) || strings.Contains(d.File, pattern) {
			return d, nil
		}
	}

	return nil, fmt.Errorf("no DAG matching %q found", pattern)
}
