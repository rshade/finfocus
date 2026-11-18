package clidocs

import (
	"strings"
)

const fence = "```"

// toStarlight adapts cobra/doc Markdown to the docs site and its markdownlint
// rules. The page title comes from front matter, so cobra's "## <command>"
// heading is dropped and its "###" sections move up one level. Bare fences get
// a language, the SEE ALSO list uses dashes and no tabs, and runs of blank
// lines outside fences collapse to one.
func toStarlight(body string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\t", "    "), "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "## ") {
		lines = lines[1:]
	}

	out := make([]string, 0, len(lines))
	inFence := false
	section := ""
	for _, line := range lines {
		if strings.HasPrefix(line, fence) {
			if !inFence && line == fence {
				line = fence + fenceLanguage(section)
			}
			inFence = !inFence
			out = append(out, line)
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}
		if line == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		var heading string
		line, heading = proseLine(line)
		if heading != "" {
			section = heading
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

// proseLine rewrites one line outside a fence. It promotes a cobra "###"
// section heading, returning the section name, and turns a SEE ALSO bullet
// into a dash item.
func proseLine(line string) (string, string) {
	if heading, ok := strings.CutPrefix(line, "### "); ok {
		title := heading
		if heading == "SEE ALSO" {
			title = "See also"
		}
		return "## " + title, heading
	}
	if item, ok := strings.CutPrefix(line, "* "); ok {
		return "- " + seeAlsoItem(item), ""
	}
	return line, ""
}

// seeAlsoItem puts a SEE ALSO entry's description on a continuation line, so
// a long command path and description stay within the line-length limit.
// cobra separates the two with a tab, already expanded to spaces here.
func seeAlsoItem(item string) string {
	link, desc, ok := strings.Cut(item, "     - ")
	if !ok {
		return item
	}
	return link + " -\n  " + desc
}

// fenceLanguage labels examples as shell and everything else (usage lines,
// flag tables, help text) as plain text.
func fenceLanguage(section string) string {
	if section == "Examples" {
		return "bash"
	}
	return "text"
}

// fencedLong wraps a command's Long help in a text fence. Help text is written
// for an 80-column terminal: it holds placeholders such as <name>, indented
// lists, and lines that start with "#", which Markdown would turn into HTML
// tags, code blocks, and headings.
func fencedLong(long string) string {
	if strings.TrimSpace(long) == "" {
		return ""
	}
	return fence + "\n" + strings.TrimRight(long, "\n") + "\n" + fence
}
