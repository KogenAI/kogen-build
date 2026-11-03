module github.com/KogenAI/kogen-build

go 1.27.2

require (
	github.com/KogenAI/kogen-coordinator v0.0.0-20261102193107-c4a49a4fe84f
	github.com/KogenAI/kogen-executor v0.0.0-20261103084720-09eabaf5b7dc
	github.com/KogenAI/kogen-review v0.0.0-20261103085153-1c3a6418c47e
	github.com/KogenAI/kogen-verification v0.0.0-20261103020618-01aa16c01767
	kogen-loop v0.0.0
)

replace kogen-loop => github.com/KogenAI/kogen-loop v0.0.0-20261102055344-c6e2360290d4
