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

	return printer.print(startStep, "", false, false)
}

// PrintString returns the DAG structure as a string
// Note: WithWriter option will be ignored if provided.
func PrintString[S any](startStep Step[S], opts ...PrintOption) (string, error) {
	var buf strings.Builder

	restOpts := make([]PrintOption, 0, len(opts))
	for _, opt := range opts {
		if _, ok := opt.(*writerOption); ok {
			continue
		}
		restOpts = append(restOpts, opt)
	}

	allOpts := append(restOpts, WithWriter(&buf))
	if err := Print(startStep, allOpts...); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// dagPrinterInterface allows steps to implement custom printing
type dagPrinterInterface[S any] interface {
	printDAG(*dagPrinter[S], string, bool, bool) error
}

// dagPrinter handles the actual printing logic
type dagPrinter[S any] struct {
	opts    *printOptions
	visited map[string]bool
}

// print is the main printing function
func (p *dagPrinter[S]) print(step Step[S], prefix string, isLast bool, needNewline bool) error {
	if step == nil {
		return p.printNil(prefix, isLast, needNewline)
	}

	// Check if step has custom printing
	if customPrinter, ok := step.(dagPrinterInterface[S]); ok {
		return customPrinter.printDAG(p, prefix, isLast, needNewline)
	}

	return p.printDefault(step, prefix, isLast, needNewline)
}

// printNil handles nil steps
func (p *dagPrinter[S]) printNil(prefix string, isLast bool, needNewline bool) error {
	if err := p.printNewLineIfNeeded(needNewline); err != nil {
		return err
	}

	connector := p.opts.connector
	if isLast {
		connector = p.opts.lastConnector
	}

	if prefix == "" {
		_, err := fmt.Fprintf(p.opts.writer, "nil")
		return err
	}
	_, err := fmt.Fprintf(p.opts.writer, "%s%snil", prefix, connector)
	return err
}

// printDefault is the standard printing for most steps
func (p *dagPrinter[S]) printDefault(step Step[S], prefix string, isLast bool, needNewline bool) error {
	name := StepName(step)
	ptr := fmt.Sprintf("%p", step)

	// Check for cycles/references
	if p.visited[ptr] {
		ref := ""
		if p.opts.showReferences {
			ref = fmt.Sprintf(" (ref to %s)", ptr)
		}
		if err := p.printNewLineIfNeeded(needNewline); err != nil {
			return err
		}
		connector := p.opts.connector
		if isLast {
			connector = p.opts.lastConnector
		}
		if prefix == "" {
			_, err := fmt.Fprintf(p.opts.writer, "%s%s", name.String(), ref)
			return err
		}
		_, err := fmt.Fprintf(p.opts.writer, "%s%s%s%s", prefix, connector, name.String(), ref)
		return err
	}
	p.visited[ptr] = true

	if err := p.printNewLineIfNeeded(needNewline); err != nil {
		return err
	}

	connector := p.opts.connector
	if isLast {
		connector = p.opts.lastConnector
	}

	if prefix == "" {
		if _, err := fmt.Fprintf(p.opts.writer, "%s", name.String()); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(p.opts.writer, "%s%s%s", prefix, connector, name.String()); err != nil {
			return err
		}
	}

	// Get children and print them
	children := unwrapper[S](step)
	childPrefix := p.getChildPrefix(prefix, isLast)

	for i, child := range children {
		isChildLast := i == len(children)-1
		if err := p.print(child, childPrefix, isChildLast, true); err != nil {
			return err
		}
	}

	return nil
}

// printWithLabel prints a step with an optional label
func (p *dagPrinter[S]) printWithLabel(step Step[S], prefix string, isLast bool, label string) error {
	if step == nil {
		if err := p.printNewLine(); err != nil {
			return err
		}
		connector := p.opts.connector
		if isLast {
			connector = p.opts.lastConnector
		}
		labelStr := ""
		if p.opts.showLabels && label != "" {
			labelStr = label
		}
		_, err := fmt.Fprintf(p.opts.writer, "%s%snil%s", prefix, connector, labelStr)
		return err
	}

	name := StepName(step)
	ptr := fmt.Sprintf("%p", step)

	if p.visited[ptr] {
		ref := ""
		if p.opts.showReferences {
			ref = fmt.Sprintf(" (ref to %s)", ptr)
		}

		if err := p.printNewLine(); err != nil {
			return err
		}
		connector := p.opts.connector
		if isLast {
			connector = p.opts.lastConnector
		}
		labelStr := ""
		if p.opts.showLabels && label != "" {
			labelStr = label
		}
		_, err := fmt.Fprintf(p.opts.writer, "%s%s%s%s%s", prefix, connector, name.String(), ref, labelStr)
		return err
	}

	// First time visiting this node
	p.visited[ptr] = true
	if err := p.printNewLine(); err != nil {
		return err
	}
	connector := p.opts.connector
	if isLast {
		connector = p.opts.lastConnector
	}

	labelStr := ""
	if p.opts.showLabels && label != "" {
		labelStr = label
	}

	if _, err := fmt.Fprintf(p.opts.writer, "%s%s%s%s", prefix, connector, name.String(), labelStr); err != nil {
		return err
	}

	// Get children and print them
	children := unwrapper[S](step)
	childPrefix := p.getChildPrefix(prefix, isLast)

	for i, child := range children {
		isChildLast := i == len(children)-1
		if err := p.print(child, childPrefix, isChildLast, true); err != nil {
			return err
		}
	}

	return nil
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
func (p *dagPrinter[S]) printNewLine() error {
	if _, err := fmt.Fprint(p.opts.writer, "\n"); err != nil {
		return err
	}
	return nil
}

func (p *dagPrinter[S]) printNewLineIfNeeded(needNewline bool) error {
	if needNewline {
		return p.printNewLine()
	}
	return nil
}

// Custom printing implementation for resultStep
func (s *resultStep[S]) printDAG(p *dagPrinter[S], prefix string, isLast bool, needNewline bool) error {
	name := StepName[S](s)
	ptr := fmt.Sprintf("%p", s)

	// Check for cycles
	if p.visited[ptr] {
		if !p.opts.showReferences {
			return nil
		}
		if err := p.printNewLineIfNeeded(needNewline); err != nil {
			return err
		}
		connector := p.opts.connector
		if isLast {
			connector = p.opts.lastConnector
		}
		if prefix == "" {
			_, err := fmt.Fprintf(p.opts.writer, "%s (ref)", name.String())
			return err
		}
		_, err := fmt.Fprintf(p.opts.writer, "%s%s%s (ref)", prefix, connector, name.String())
		return err
	}
	p.visited[ptr] = true

	if err := p.printNewLineIfNeeded(needNewline); err != nil {
		return err
	}

	connector := p.opts.connector
	if isLast {
		connector = p.opts.lastConnector
	}

	if prefix == "" {
		if _, err := fmt.Fprintf(p.opts.writer, "%s", name.String()); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(p.opts.writer, "%s%s%s", prefix, connector, name.String()); err != nil {
			return err
		}
	}

	// Calculate child prefix
	childPrefix := p.getChildPrefix(prefix, isLast)

	// Print main step with [main] label
	if err := p.printWithLabel(s.mainStep, childPrefix, false, " [main]"); err != nil {
		return err
	}

	// Print success step with [success] label
	hasFailureHandler := s.failureHandler != nil
	if err := p.printWithLabel(s.successStep, childPrefix, !hasFailureHandler, " [success]"); err != nil {
		return err
	}

	// Handle failure handler
	if s.failureHandler != nil {
		// For single failure handlers, print them directly with [failure] label
		failureSteps := unwrapper[S](s.failureHandler)
		if len(failureSteps) == 1 {
			if err := p.printWithLabel(failureSteps[0], childPrefix, true, " [failure]"); err != nil {
				return err
			}
		} else {
			// Multiple failure steps (shouldn't normally happen, but handle it)
			for i, fs := range failureSteps {
				isLastFailure := i == len(failureSteps)-1
				if err := p.printWithLabel(fs, childPrefix, isLastFailure, " [failure]"); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
