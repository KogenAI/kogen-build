package build

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KogenAI/kogen-verification/verify"
)

type unmovedWatch struct {
	stage, tree string
	items       []string
	streak      int
}

// failingItems separates scenario membership from the aggregate acceptance gate. The
// corresponding texts are kept separately: changing a message does not change identity.
func failingItems(pass verify.Pass) ([]string, []string) {
	var ids, texts []string
	scenarios := 0
	if pass.Acceptance != nil {
		for _, s := range pass.Acceptance.Scenarios {
			if s.Status == "passed" {
				continue
			}
			scenarios++
			ids = append(ids, fmt.Sprintf("scenario:%d:%s", s.Line, s.Name))
			text := ""
			if s.Detail != nil {
				_, text, _ = strings.Cut(*s.Detail, "\n")
			}
			texts = append(texts, text)
		}
	}
	for _, name := range pass.Failing {
		if name == "acceptance" && scenarios > 0 {
			continue
		}
		ids = append(ids, "gate:"+name)
		text := ""
		for _, c := range append(slices.Clone(pass.Setup), pass.Checks...) {
			if c.Name == name && c.Status != 0 {
				text = c.Output
				break
			}
		}
		if name == "model" {
			text = strings.Join(pass.Model.Problems, "\n")
		}
		for _, m := range pass.MaintainedModels {
			if m.Path == name {
				text = strings.Join(m.Result.Problems, "\n")
			}
		}
		if name == "acceptance" && pass.Acceptance != nil {
			text = pass.Acceptance.Output
		}
		texts = append(texts, text)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	return ids, texts
}

func commonFailure(texts []string) string {
	counts := map[string]int{}
	for _, text := range texts {
		if text != "" {
			counts[text]++
			if counts[text] > len(texts)/2 {
				return text
			}
		}
	}
	return ""
}

func (b *Attempt) unmovedNotice(stage string, pass verify.Pass) string {
	w := &b.unmoved
	if pass.Green {
		*w = unmovedWatch{}
		return ""
	}
	ids, texts := failingItems(pass)
	if len(ids) == 0 {
		*w = unmovedWatch{}
		return ""
	}
	same := w.streak > 0 && w.stage == stage && slices.Equal(w.items, ids)
	oldTree := w.tree
	if same {
		w.streak++
	} else {
		w.streak = 1
	}
	w.stage, w.tree, w.items = stage, pass.Tree, ids
	if !same || oldTree == "" || pass.Tree == "" || oldTree == pass.Tree {
		return ""
	}
	paths, err := b.repo.Changed(oldTree, pass.Tree)
	if err != nil || len(paths) == 0 {
		return ""
	}
	slices.Sort(paths)
	notice := fmt.Sprintf("Failing set did not move for %d consecutive red passes. Last round changed: %s. None of the failing items changed.", w.streak, strings.Join(paths, ", "))
	if common := commonFailure(texts); common != "" {
		notice += "\nCommon failure text (address first): " + common
	}
	return notice
}
