package build

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-loop/sys"
)

// promoteModel publishes the approved specification as a complete directory.
func (b *Attempt) promoteModel() error {
	m := b.intent.Model
	if m == nil || m.None {
		return nil
	}
	dir := filepath.Join(b.Root, ".kogen/models", b.Slug)
	refuse := func(err error) error {
		return sys.Fail("candidate", "model_promotion_refused", fmt.Sprintf("model directory %s: %v", dir, err), 1)
	}
	info, err := os.Lstat(dir)
	if err == nil {
		if !info.IsDir() {
			return refuse(fmt.Errorf("not a directory"))
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return refuse(err)
		}
		if len(entries) != 0 {
			if len(entries) != 2 || entries[0].Name() != "model.json" || entries[1].Name() != "model.qnt" || !entries[0].Type().IsRegular() || !entries[1].Type().IsRegular() {
				return refuse(fmt.Errorf("expected exactly model.json and model.qnt regular files"))
			}
			data, err := os.ReadFile(filepath.Join(dir, "model.json"))
			if err != nil {
				return refuse(err)
			}
			var spec project.ModelSpec
			if err := json.Unmarshal(data, &spec); err != nil {
				return refuse(err)
			}
			if spec.Path != filepath.ToSlash(filepath.Join(".kogen/models", b.Slug, "model.qnt")) {
				return refuse(fmt.Errorf("model.json names a different model"))
			}
			return nil
		}
	} else if !os.IsNotExist(err) {
		return refuse(err)
	}
	if err := b.approvalIntact(); err != nil {
		return refuse(err)
	}
	if err := b.BaselineIntact(); err != nil {
		return refuse(err)
	}
	data, err := os.ReadFile(filepath.Join(b.Root, m.Path))
	if err != nil {
		return refuse(err)
	}
	sum := sha256.Sum256(data)
	digest, ok := b.BaselineDigests[m.Path]
	if !ok || len(data) == 0 || digest != hex.EncodeToString(sum[:]) {
		return refuse(fmt.Errorf("approved model source is missing, empty or changed"))
	}
	spec := *m
	spec.Path = filepath.ToSlash(filepath.Join(".kogen/models", b.Slug, "model.qnt"))
	record, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return refuse(err)
	}
	local := filepath.Join(b.Root, ".kogen/local")
	if err := os.MkdirAll(local, 0755); err != nil {
		return refuse(err)
	}
	stage, err := os.MkdirTemp(local, "model-promotion-")
	if err != nil {
		return refuse(err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0755); err != nil {
		return refuse(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "model.qnt"), data, 0644); err != nil {
		return refuse(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "model.json"), append(record, '\n'), 0644); err != nil {
		return refuse(err)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return refuse(err)
	}
	if err := syscall.Rename(stage, dir); err != nil {
		return refuse(err)
	}
	return nil
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
