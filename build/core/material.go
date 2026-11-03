package core

// MaterialPartBytes is the most source bytes in one part of the reviewer's pinned source (and
// the room an external review's mandatory prompt must leave for source).
const MaterialPartBytes = 6000

// MaterialSet is what the reviewer reads about one green pass.
type MaterialSet struct {
	// Authority is the stable part: the approved contract and the commands.
	Authority string `json:"authority"`
	// Source is the dynamic part: the check results, the diff and the file texts.
	Source string `json:"source"`
	// Files is the catalog of changed paths: path, status (changed, added or deleted), Git
	// mode, Git object, byte count and SHA-256 of the content (the commit id for a gitlink).
	Files []map[string]any `json:"files"`
	// Checks are the check results of the final pass.
	Checks string `json:"checks"`
	// Manifest (on_demand only) is, per Files entry, its diff_stat and hunks; Segments locate each
	// path's diff block and file text in Source.
	Manifest []map[string]any  `json:"manifest,omitempty"`
	Segments []MaterialSegment `json:"segments,omitempty"`
	// Hunks (on_demand only) are the lines each changed hunk occupies: the pinned parts that
	// cover one of them are required reads before a verdict.
	Hunks []MaterialHunkRange `json:"hunk_ranges,omitempty"`
	// Texts are the changed or added text files as they appear in Source (the external
	// reviewer's parts keep a path's text whole). Not persisted.
	Texts []MaterialFileText `json:"-"`
	// PartsSource is the source of the parts route (an external review whose complete prompt does
	// not fit one session): Source with the full text of every changed text file left out, and
	// PartsTexts the added text files that remain in it. Not persisted.
	PartsSource string             `json:"-"`
	PartsTexts  []MaterialFileText `json:"-"`
}

// MaterialHunkRange is the lines (1-based, inclusive) of one changed hunk: kind "diff" within
// its path's diff block (its header and body), kind "file" within the candidate text of its path
// (the hunk's new lines; a hunk that only deletes has none).
type MaterialHunkRange struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	First int    `json:"first_line"`
	Last  int    `json:"last_line"`
}

// MaterialSegment is the byte range [Start, End) of Source holding the diff block (kind
// "diff") or the text (kind "file") of one path.
type MaterialSegment struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// MaterialFileText is a changed or added text file as it appears in the source.
type MaterialFileText struct{ Path, Heading, Content string }
