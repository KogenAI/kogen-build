package build

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/KogenAI/kogen-coordinator/project"
	"github.com/KogenAI/kogen-executor/core"
	"github.com/KogenAI/kogen-review/review"
	"github.com/KogenAI/kogen-verification/verify"
)

// What build shares with kogen-review lives in kogen-executor's core (the token totals, the
// review record, an external executor's state) and in kogen-review (the review material, which
// the review makes and the Build keeps in the review's working window). build re-exports them
// under its own names.
type (
	Tokens        = core.Tokens
	ReviewRecord  = core.ReviewRecord
	ExecutorState = core.ExecutorState
	ExecutorUsage = core.ExecutorUsage

	materialSet = review.MaterialSet

	// invalidReview is a review that ended without a valid verdict, as kogen-review returns it.
	invalidReview = review.Invalid
)

// Approved is the result of a review that approves: the green pass whose exact tree was
// reviewed (the final one, after any repairs the reviewer made and verification repeated) and
// the review.
type Approved struct {
	Pass   verify.Pass
	Review ReviewRecord
}

// RequestSpec is one model request of a Build.
type RequestSpec struct {
	// Owner is the conversation: "build" or "review".
	Owner        string
	Role         project.Role
	Instructions string
	Input        []any
	Tools        []any
	// Retain records the input actually sent (after working-context resets) in the
	// continuation the request is checkpointed with. Nil: the builder's Continuation.Input.
	Retain func(c *Continuation, input []any)
	// Prefill, when set, may append native data to the input after the working window is
	// prepared and before the request is checkpointed, so the checkpoint, the archive and the
	// request all hold exactly what is sent.
	Prefill func(input []any) ([]any, error)
	// ForceTool names a tool this request alone must call, where the route supports named
	// tool selection (provider.Request.ToolChoice); the next request is ordinary again.
	ForceTool string
}

// decodeBase64 decodes standard base64.
func decodeBase64(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

// decodeJSONNumbers decodes one JSON value keeping numbers as json.Number.
func decodeJSONNumbers(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data")
	}
	return v, nil
}
