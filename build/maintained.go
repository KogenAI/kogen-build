package build

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/KogenAI/kogen-coordinator/project"
)

// promoteModel creates the candidate's independent project specification once.
func (b *Attempt) promoteModel() error {
	m := b.intent.Model
	if m == nil || m.None {
		return nil
	}
	dir := filepath.Join(b.Root, ".kogen/models", b.Slug)
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(filepath.Join(b.Root, m.Path))
	if err != nil {
		return err
	}
	spec := *m
	spec.Path = filepath.ToSlash(filepath.Join(".kogen/models", b.Slug, "model.qnt"))
	record, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(b.Root, spec.Path), data, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "model.json"), append(record, '\n'), 0644)
}

func maintainedModels(root string) ([]project.ModelSpec, error) {
	dir := filepath.Join(root, ".kogen/models")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var models []project.ModelSpec
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name(), "model.json"))
		if err != nil {
			return nil, err
		}
		var spec project.ModelSpec
		if err = json.Unmarshal(data, &spec); err != nil {
			return nil, err
		}
		models = append(models, spec)
	}
	return models, nil
}
