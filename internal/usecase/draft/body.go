package draft

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"path/filepath"
	"strings"

	"json-inspector/internal/domain"
)

// renderBody is one body as the text that goes on the wire.
//
// `masked` renders the copy that outlives the send: a file keeps its name and loses its bytes, so a
// record is a description of the upload rather than a second copy of the file.
//
// `used` is the boundary that was actually written, and is empty for every kind but a form. It is
// not part of the body — it is what the body's delimiter turned out to be — and it comes back from
// here so that the header naming it cannot disagree with the text that carries it.
//
// A boundary handed in is reused rather than minted, which is what lets the live and the written
// copy be two renderings of one encoding instead of two encodings that happen to look alike.
func (u *UseCase) renderBody(kind domain.BodyKind, text string, file string, rows []domain.FormRow, boundary string, masked bool) (body string, used string, err error) {
	switch kind {
	case domain.BodyForm:
		return u.renderForm(rows, boundary, masked)
	case domain.BodyBinary:
		// The path and not the text: a binary body is the file, and what is in the text box beside it
		// is whatever the user last typed as JSON or Raw.
		return u.renderFile(file, masked)
	default:
		return text, "", nil
	}
}

// renderForm is the Form-Data grid as a multipart body. Only the rows that are switched on and named
// go out: a blank key is a part no server can read.
func (u *UseCase) renderForm(rows []domain.FormRow, boundary string, masked bool) (string, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if boundary != "" {
		// An invalid boundary can only come from here, so a failure is a bug rather than a state.
		if err := writer.SetBoundary(boundary); err != nil {
			return "", "", fmt.Errorf("form boundary: %w", err)
		}
	}
	used := writer.Boundary()

	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if !row.Enabled || name == "" {
			continue
		}
		if row.File {
			if err := writeFormFile(writer, row, u, masked); err != nil {
				return "", "", err
			}
			continue
		}
		if err := writer.WriteField(name, row.Value); err != nil {
			return "", "", fmt.Errorf("form field %q: %w", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", "", fmt.Errorf("closing the form: %w", err)
	}
	return buf.String(), used, nil
}

// writeFormFile is one file part. CreatePart is used rather than CreateFormFile because the latter
// hardcodes application/octet-stream, and the type of each part is the point of picking a file.
func writeFormFile(writer *multipart.Writer, row domain.FormRow, u *UseCase, masked bool) error {
	name := strings.TrimSpace(row.Name)
	file := row.Src

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		escapeQuotes(name), escapeQuotes(filepath.Base(file))))
	header.Set("Content-Type", typeOfFile(file))

	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("form part %q: %w", name, err)
	}
	// The masked copy names the file and carries nothing: the bytes are already on their way, and a
	// record that held them would be a second copy of the upload.
	if masked {
		return nil
	}
	data, err := u.files.Read(file)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return nil
}

// renderFile is a whole file as the body. The masked copy is empty for the reason a masked part is.
func (u *UseCase) renderFile(path string, masked bool) (string, string, error) {
	if masked || path == "" {
		return "", "", nil
	}
	data, err := u.files.Read(path)
	if err != nil {
		return "", "", err
	}
	return string(data), "", nil
}

// typeOfFile is what a part or a whole body declares itself as. The extension is all there is to go
// on without opening the file, and it is enough — it is what every client uses. An unknown one still
// declares the bytes rather than nothing at all.
func typeOfFile(path string) string {
	if guessed := mime.TypeByExtension(filepath.Ext(path)); guessed != "" {
		return guessed
	}
	return "application/octet-stream"
}

// escapeQuotes is what multipart uses for a name or a filename it writes into a header: a quote in a
// file name would otherwise end the parameter and start something else.
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}
