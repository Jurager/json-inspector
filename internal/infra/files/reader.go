// Package files reads the bytes a request carries. It is here and not in a usecase because a
// scenario may not touch the outside world: opening a path is exactly that, and the feature that
// needs it asks for an interface it declares itself.
package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"

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
// corrupt file at the other end, which is worse than a send that did not happen. The extra byte
// read past the limit is what tells "ends exactly at the limit" from "was cut there".
func (r *Reader) Read(path string) ([]byte, error) {
	limit := r.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, domain.Refuse(domain.CodeFileMissing, domain.ErrNotFound, domain.Args{"path": path})
		}
		return nil, domain.Refuse(domain.CodeFileUnreadable, domain.ErrNotAllowed,
			domain.Args{"path": path, "reason": err.Error()})
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, domain.Refuse(domain.CodeFileUnreadable, domain.ErrNotAllowed,
			domain.Args{"path": path, "reason": err.Error()})
	}
	if int64(len(data)) > limit {
		// The limit is named in the unit its sentence names, so the catalogue never has to say "bytes".
		return nil, domain.Refuse(domain.CodeFileTooLarge, domain.ErrNotAllowed, domain.Args{
			"path":  path,
			"limit": strconv.Itoa(int(limit >> 20)),
		})
	}
	return data, nil
}
