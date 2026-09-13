package dagger

import (
	"io"
	"os"
)

// PrintOption configures the DAG printing behavior
type PrintOption interface {
	apply(*printOptions)
}

// printOptions holds all configuration for DAG printing
type printOptions struct {
	writer         io.Writer
	connector      string // Symbol for non-last children
	lastConnector  string // Symbol for last child
	indent         string // Base indentation (e.g., "    ")
	verticalLine   string // Continuation line for non-last children (e.g., "│   ")
	emptySpace     string // Space for last children's descendants (e.g., "    ")
	showReferences bool   // Whether to show "(ref)" for repeated nodes
	showLabels     bool   // Whether to show labels like [main], [success], [failure]
}

// defaultPrintOptions returns the default printing configuration
func defaultPrintOptions() *printOptions {
	return &printOptions{
		writer:         os.Stdout,
		connector:      "├── ",
		lastConnector:  "└── ",
		indent:         "    ",
		verticalLine:   "│   ",
		emptySpace:     "    ",
		showReferences: false,
		showLabels:     true,
	}
}

type writerOption struct{ w io.Writer }

func (o *writerOption) apply(opts *printOptions) {
	opts.writer = o.w
}

// WithWriter sets the output writer (default: os.Stdout)
func WithWriter(w io.Writer) PrintOption {
	return &writerOption{w}
}

type symbolsOption struct {
	connector     string
	lastConnector string
	verticalLine  string
	emptySpace    string
}

func (o symbolsOption) apply(opts *printOptions) {
	if o.connector != "" {
		opts.connector = o.connector
	}
	if o.lastConnector != "" {
		opts.lastConnector = o.lastConnector
	}
	if o.verticalLine != "" {
		opts.verticalLine = o.verticalLine
	}
	if o.emptySpace != "" {
		opts.emptySpace = o.emptySpace
	}
}

// WithSymbols customizes the tree symbols
func WithSymbols(connector, lastConnector, verticalLine, emptySpace string) PrintOption {
	return symbolsOption{
		connector:     connector,
		lastConnector: lastConnector,
		verticalLine:  verticalLine,
		emptySpace:    emptySpace,
	}
}

// WithCompactSymbols uses more compact ASCII symbols
func WithCompactSymbols() PrintOption {
	return symbolsOption{
		connector:     "|- ",
		lastConnector: "`- ",
		verticalLine:  "|  ",
		emptySpace:    "   ",
	}
}

// WithIndent sets the base indentation string
func WithIndent(indent string) PrintOption {
	return indentOption{indent}
}

type indentOption struct{ indent string }

func (o indentOption) apply(opts *printOptions) {
	opts.indent = o.indent
	// Also update vertical line and empty space to match indent length
	if o.indent != "" {
		opts.verticalLine = "│" + o.indent[1:]
		opts.emptySpace = o.indent
	}
}

type showReferencesOption struct{ show bool }

func (o showReferencesOption) apply(opts *printOptions) {
	opts.showReferences = o.show
}

// WithReferences enables showing "(ref)" for repeated nodes
func WithReferences() PrintOption {
	return showReferencesOption{true}
}

type showLabelsOption struct{ show bool }

func (o showLabelsOption) apply(opts *printOptions) {
	opts.showLabels = o.show
}

// WithoutLabels hides branch labels like [main], [success], [failure]
func WithoutLabels() PrintOption {
	return showLabelsOption{false}
}
