package dagger

import (
	"fmt"
	"strings"
)

// Print outputs the DAG structure with the given options
func Print[S any](startStep Step[S], opts ...PrintOption) error {
	options := defaultPrintOptions()
	for _, opt := range opts {
		opt.apply(options)
	}

	printer := &dagPrinter[S]{
		opts:    options,
		visited: make(map[string]bool),
	}

	printer.print(startStep, "", false, false)
	return nil
}

// PrintString returns the DAG structure as a string.
// Note: WithWriter option will be ignored if provided.
func PrintString[S any](startStep Step[S], opts ...PrintOption) string {
	var buf strings.Builder

	options := defaultPrintOptions()
	for _, opt := range opts {
		if _, ok := opt.(*writerOption); ok {
			continue
		}
		opt.apply(options)
	}
	options.writer = &buf

	printer := &dagPrinter[S]{
		opts:    options,
		visited: make(map[string]bool),
	}

	printer.print(startStep, "", false, false)
	return buf.String()
}

// dagPrinterInterface allows steps to implement custom printing
type dagPrinterInterface[S any] interface {
	printDAG(*dagPrinter[S], string, bool, bool)
}

// dagPrinter handles the actual printing logic
type dagPrinter[S any] struct {
	opts    *printOptions
	visited map[string]bool
}

func (p *dagPrinter[S]) connector(isLast bool) string {
	if isLast {
		return p.opts.lastConnector
	}
	return p.opts.connector
}

// print is the main printing function
func (p *dagPrinter[S]) print(step Step[S], prefix string, isLast bool, needNewline bool) {
	if step == nil {
		p.printNil(prefix, isLast, needNewline)
		return
	}

	// Check if step has custom printing
	if customPrinter, ok := step.(dagPrinterInterface[S]); ok {
		customPrinter.printDAG(p, prefix, isLast, needNewline)
		return
	}

	p.printDefault(step, prefix, isLast, needNewline)
}

// printNil handles nil steps
func (p *dagPrinter[S]) printNil(prefix string, isLast bool, needNewline bool) {
	p.printNewLineIfNeeded(needNewline)

	if prefix == "" {
		fmt.Fprintf(p.opts.writer, "nil")
		return
	}
	fmt.Fprintf(p.opts.writer, "%s%snil", prefix, p.connector(isLast))
}

// printNodeHeader prints the prefix, connector, and name for a node.
// It returns true if this is the first visit to the node (caller should print children),
// or false if the node was already visited (caller should not print children).
func (p *dagPrinter[S]) printNodeHeader(step Step[S], prefix string, isLast bool, needNewline bool) bool {
	name := StepName(step)
	ptr := fmt.Sprintf("%p", step)

	if p.visited[ptr] {
		p.printNewLineIfNeeded(needNewline)
		ref := ""
		if p.opts.showReferences {
			ref = " (ref)"
		}
		fmt.Fprintf(p.opts.writer, "%s%s%s%s", prefix, p.connector(isLast), name.String(), ref)
		return false
	}
	p.visited[ptr] = true

	p.printNewLineIfNeeded(needNewline)

	if prefix == "" {
		fmt.Fprintf(p.opts.writer, "%s", name.String())
		return true
	}

	fmt.Fprintf(p.opts.writer, "%s%s%s", prefix, p.connector(isLast), name.String())
	return true
}

// printDefault is the standard printing for most steps
func (p *dagPrinter[S]) printDefault(step Step[S], prefix string, isLast bool, needNewline bool) {
	if !p.printNodeHeader(step, prefix, isLast, needNewline) {
		return
	}

	// Get children and print them
	children := unwrapper[S](step)
	childPrefix := p.getChildPrefix(prefix, isLast)

	for i, child := range children {
		isChildLast := i == len(children)-1
		p.print(child, childPrefix, isChildLast, true)
	}
}

// printWithLabel prints a step with an optional label
func (p *dagPrinter[S]) printWithLabel(step Step[S], prefix string, isLast bool, label string) {
	p.printNewLine()

	connector := p.connector(isLast)
	labelStr := ""
	if p.opts.showLabels && label != "" {
		labelStr = label
	}

	if step == nil {
		fmt.Fprintf(p.opts.writer, "%s%snil%s", prefix, connector, labelStr)
		return
	}

	name := StepName(step)
	ptr := fmt.Sprintf("%p", step)

	if p.visited[ptr] {
		ref := ""
		if p.opts.showReferences {
			ref = " (ref)"
		}
		fmt.Fprintf(p.opts.writer, "%s%s%s%s%s", prefix, connector, name.String(), ref, labelStr)
		return
	}

	p.visited[ptr] = true
	fmt.Fprintf(p.opts.writer, "%s%s%s%s", prefix, connector, name.String(), labelStr)

	// Get children and print them
	children := unwrapper[S](step)
	childPrefix := p.getChildPrefix(prefix, isLast)

	for i, child := range children {
		isChildLast := i == len(children)-1
		p.print(child, childPrefix, isChildLast, true)
	}
}

// getChildPrefix calculates the prefix for child nodes
func (p *dagPrinter[S]) getChildPrefix(parentPrefix string, parentIsLast bool) string {
	if parentPrefix == "" {
		return p.opts.indent
	}
	if parentIsLast {
		return parentPrefix + p.opts.emptySpace
	}
	return parentPrefix + p.opts.verticalLine
}

// printNewLine prints a newline
func (p *dagPrinter[S]) printNewLine() {
	fmt.Fprint(p.opts.writer, "\n")
}

func (p *dagPrinter[S]) printNewLineIfNeeded(needNewline bool) {
	if needNewline {
		p.printNewLine()
	}
}

// Custom printing implementation for resultStep
func (s *resultStep[S]) printDAG(p *dagPrinter[S], prefix string, isLast bool, needNewline bool) {
	if !p.printNodeHeader(s, prefix, isLast, needNewline) {
		return
	}

	// Calculate child prefix
	childPrefix := p.getChildPrefix(prefix, isLast)

	// Print main step with [main] label
	p.printWithLabel(s.mainStep, childPrefix, false, " [main]")

	// Print success step with [success] label
	hasFailureHandler := s.failureHandler != nil
	p.printWithLabel(s.successStep, childPrefix, !hasFailureHandler, " [success]")

	// Handle failure handler
	if s.failureHandler != nil {
		failureSteps := unwrapper[S](s.failureHandler)
		for i, fs := range failureSteps {
			isLastFailure := i == len(failureSteps)-1
			p.printWithLabel(fs, childPrefix, isLastFailure, " [failure]")
		}
	}
}
