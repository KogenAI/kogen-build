module github.com/KogenAI/kogen-build

go 1.27.2

require (
	github.com/KogenAI/kogen-coordinator v0.0.0-20261103113131-ffca7072fe15
	github.com/KogenAI/kogen-executor v0.0.0-20261103084720-09eabaf5b7dc
	github.com/KogenAI/kogen-review v0.0.0-20261103113204-11d83000d2f2
	github.com/KogenAI/kogen-verification v0.0.0-20261103020618-01aa16c01767
	kogen-loop v0.0.0
)

replace kogen-loop => github.com/KogenAI/kogen-loop v0.0.0-20261102055344-c6e2360290d4
