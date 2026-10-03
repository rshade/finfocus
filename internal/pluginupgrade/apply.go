package pluginupgrade

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

type edit struct {
	span

	replacement string
}

type pendingWrite struct {
	rel  string
	data []byte
}

// Apply performs the plan's automatic edits on the project Detect returned:
// SpecVersion declarations, and go.mod's spec requirement and go directive. Nothing else is changed; byte spans outside
// the edited literals are preserved exactly. Every file is checked against
// what Detect read before any file is written, and go.mod is written last.
// It returns the changed files relative to the project directory.
func Apply(p *Project, plan *Plan) ([]string, error) {
	if plan.UpToDate {
		return nil, nil
	}

	rels := make([]string, 0, len(p.sources))
	for rel := range p.sources {
		rels = append(rels, rel)
	}
	sort.Strings(rels)

	var writes []pendingWrite
	for _, rel := range rels {
		src := p.sources[rel]
		if err := p.checkUnchanged(rel, src.content); err != nil {
			return nil, err
		}
		updated := applyEdits(src.content, sourceEdits(src, plan))
		if !bytes.Equal(updated, src.content) {
			writes = append(writes, pendingWrite{rel: rel, data: updated})
		}
	}

	if err := p.checkUnchanged("go.mod", p.goMod); err != nil {
		return nil, err
	}
	goMod, err := rewriteGoMod(p, plan)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(goMod, p.goMod) {
		writes = append(writes, pendingWrite{rel: "go.mod", data: goMod})
	}

	changed := make([]string, 0, len(writes))
	for _, w := range writes {
		if writeErr := writePreservingMode(filepath.Join(p.Dir, filepath.FromSlash(w.rel)), w.data); writeErr != nil {
			if len(changed) == 0 {
				return changed, fmt.Errorf("writing %s: %w", w.rel, writeErr)
			}
			return changed, fmt.Errorf("writing %s (already updated: %s): %w",
				w.rel, strings.Join(changed, ", "), writeErr)
		}
		changed = append(changed, w.rel)
	}
	sort.Strings(changed)
	return changed, nil
}

func (p *Project) checkUnchanged(rel string, scanned []byte) error {
	current, err := os.ReadFile(filepath.Join(p.Dir, filepath.FromSlash(rel)))
	if err != nil {
		return fmt.Errorf("reading %s: %w", rel, err)
	}
	if !bytes.Equal(current, scanned) {
		return fmt.Errorf("%s changed since it was scanned; run the upgrade again", rel)
	}
	return nil
}

func sourceEdits(src *source, plan *Plan) []edit {
	var edits []edit
	for _, s := range src.decls {
		if s.value != plan.Target {
			edits = append(edits, edit{span: s, replacement: strconv.Quote(plan.Target)})
		}
	}
	return edits
}

// applyEdits replaces non-overlapping spans, working backwards so earlier
// offsets stay valid.
func applyEdits(content []byte, edits []edit) []byte {
	if len(edits) == 0 {
		return content
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), content...)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.replacement), out[e.end:]...)...)
	}
	return out
}

func rewriteGoMod(p *Project, plan *Plan) ([]byte, error) {
	mf, err := modfile.Parse("go.mod", p.goMod, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing go.mod: %w", err)
	}
	if err = mf.AddRequire(Module, plan.Target); err != nil {
		return nil, fmt.Errorf("setting %s %s in go.mod: %w", Module, plan.Target, err)
	}
	if plan.RequiredGo != "" {
		if err = mf.AddGoStmt(plan.RequiredGo); err != nil {
			return nil, fmt.Errorf("setting go %s in go.mod: %w", plan.RequiredGo, err)
		}
	}
	mf.Cleanup()
	out, err := mf.Format()
	if err != nil {
		return nil, fmt.Errorf("formatting go.mod: %w", err)
	}
	return out, nil
}

func writePreservingMode(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, info.Mode().Perm())
}
