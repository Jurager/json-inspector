// Package files reads the bytes a request carries. It is here and not in a usecase because a
// scenario may not touch the outside world: opening a path is exactly that, and the feature that
// needs it asks for an interface it declares itself.
package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"json-inspector/internal/domain"
)

// Reader reads a file whole, up to a limit. Zero means the default.
type Reader struct {
	MaxBytes int64
}

// DefaultMaxBytes is what a body is capped at when nothing says otherwise. It is the same ceiling
// the engine reads a response under, because a request and its answer are the same size of problem.
const DefaultMaxBytes int64 = 8 << 20

func NewReader() *Reader {
	return &Reader{MaxBytes: DefaultMaxBytes}
}

// Read answers with the file's bytes.
//
// A file larger than the limit is an error and not a truncation. A truncated response is still
// something to read — it says so, and the window draws a partial body — but a truncated upload is a
// corrupt file at the other end, which is worse than a send that did not happen. The extra byte read
// past the limit is what tells "ends exactly at the limit" from "was cut there".
func (r *Reader) Read(path string) ([]byte, error) {
	limit := r.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file %s: %w", path, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file %s is over %d bytes: %w", path, limit, domain.ErrNotAllowed)
	}
	return data, nil
}
