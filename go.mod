module github.com/KogenAI/kogen-build

go 1.27.2

require (
	github.com/KogenAI/kogen-coordinator v0.0.0-20261103113131-ffca7072fe15
	github.com/KogenAI/kogen-executor v0.0.0-20261103084720-09eabaf5b7dc
	github.com/KogenAI/kogen-review v0.0.0-20261103124347-cfc2fb29917c
	github.com/KogenAI/kogen-verification v0.0.0-20261103020618-01aa16c01767
	kogen-loop v0.0.0
)

replace kogen-loop => github.com/KogenAI/kogen-loop v0.0.0-20261103193244-a5aa5f4ee775
