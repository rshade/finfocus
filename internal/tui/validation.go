package tui

import (
	"fmt"
	"strings"
)

// ValidationView is the styled or plain rendering of a configuration check.
type ValidationView struct {
	File     string
	Valid    bool
	Errors   []ValidationViewItem
	Warnings []ValidationViewItem
}

// ValidationViewItem is one error or warning line in a validation view.
type ValidationViewItem struct {
	File       string
	Line       int
	Path       string
	Message    string
	Hint       string
	Example    string
	Suggestion string
}

// RenderValidationReport renders view with the shared box style, or as plain text when plain is set.
func RenderValidationReport(view ValidationView, plain bool) string {
	if plain {
		return renderPlainValidation(view)
	}
	return BoxStyle.Render(renderPlainValidation(view))
}

func renderPlainValidation(view ValidationView) string {
	var b strings.Builder
	if view.Valid {
		b.WriteString("Configuration Validation\n")
	} else {
		b.WriteString("Configuration Error")
		if view.File != "" {
			b.WriteString(": " + view.File)
		}
		b.WriteByte('\n')
	}
	if view.File != "" && view.Valid {
		b.WriteString("File: " + view.File + "\n")
	}
	if view.Valid && len(view.Errors) == 0 {
		b.WriteString("Syntax valid\n")
		b.WriteString("Budget configuration valid\n")
	}
	writeValidationItems(&b, "Error", view.Errors)
	writeValidationItems(&b, "Warning", view.Warnings)
	if view.Valid && len(view.Warnings) == 0 {
		b.WriteString("Configuration is valid\n")
	}
	return b.String()
}

func writeValidationItems(b *strings.Builder, label string, items []ValidationViewItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%ss:\n", label)
	for _, item := range items {
		if item.Line > 0 {
			fmt.Fprintf(b, "  %s at line %d: %s\n", label, item.Line, item.Message)
		} else {
			fmt.Fprintf(b, "  %s: %s\n", label, item.Message)
		}
		if item.File != "" {
			fmt.Fprintf(b, "  File: %s\n", item.File)
		}
		if item.Path != "" {
			fmt.Fprintf(b, "  Path: %s\n", item.Path)
		}
		if item.Hint != "" {
			fmt.Fprintf(b, "  Hint: %s\n", item.Hint)
		}
		if item.Suggestion != "" {
			fmt.Fprintf(b, "  %s\n", item.Suggestion)
		}
		if item.Example != "" {
			fmt.Fprintf(b, "  Example:\n  %s\n", item.Example)
		}
	}
}
