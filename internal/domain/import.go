package domain

// ImportStatus is how far a one-time import of what an older version left behind has got.
//
// It is here rather than next to the table it is written to because the two sides sit in different
// layers: the store keeps the row, and the feature that does the importing decides the status — and
// a feature may not reach into the store's package for a word. Written as a type rather than as
// three strings so that the five places that name one cannot misspell it.
type ImportStatus string

const (
	ImportPending ImportStatus = "pending"
	ImportDone    ImportStatus = "done"
	ImportFailed  ImportStatus = "failed"
)
