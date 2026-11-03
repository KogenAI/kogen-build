package contract_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/KogenAI/kogen-build/build"
	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/executor"
	"github.com/KogenAI/kogen-verification/verify"
	"kogen-loop/provider"
	"kogen-loop/sys"
)

// These calls mirror the assembled consumer's entry points and compile independently
// of the producer's internal tests.
func consumerCalls(ctx context.Context, session *sys.Session, client *provider.Client, tools verify.Tools, externalExecutor func(project.Role) (executor.Executor, error), project, slug, id, output, path string, data []byte) (int, error) {
	diagnostic := &build.Diagnostic{Text: string(data), SourcePath: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	var code int
	var err error
	code, err = build.Build(ctx, session, build.Input{Deps: build.Deps{Client: client, Tools: tools, External: externalExecutor}, Project: project, Slug: slug, Context: diagnostic})
	code, err = build.Resume(ctx, session, build.ResumeInput{Deps: build.Deps{Client: client, Tools: tools, External: externalExecutor}, Project: project, ID: id})
	code, err = build.Recover(ctx, session, build.RecoverInput{Project: project, ID: id, Output: output})
	return code, err
}

func TestDiagnosticConstruction(t *testing.T) {
	data := []byte("diagnostic")
	d := build.Diagnostic{Text: string(data), SourcePath: "context.txt", SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	if d.Text != string(data) || d.SourcePath != "context.txt" || d.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal(d)
	}
}
