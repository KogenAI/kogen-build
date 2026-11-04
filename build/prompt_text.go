package build

// The two fixed passages of the builder's opening message (chapter 04).
const (
	protectedIntro = `These files are the approved acceptance baseline. Approval freezes these bytes; it is not an exception or authorization to change them. write_file refuses them, and if any of them changes the Build stops at once without a commit:`

	manifestNotice = `Dependency manifests and lockfiles that are not in this baseline remain editable inputs for approved implementation changes. Their editable portions do not authorize weakening, removing or bypassing a configured check or changing actual verification configuration. Preserve all configured checks, their commands and the behavior they verify. Protection is whole-file: if a combined check-definition manifest is listed above, it cannot be edited for dependencies either.`
)

// finishReminder ends the feedback of a red whole-Intent pass that goes back to the builder.
const finishReminder = `Reminder: finish means the whole approved Intent is implemented and every behavior it requires works through the real entry points that a user or an acceptance scenario reaches; helper-only or partially wired work will be red again.`
