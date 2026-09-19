package record

import (
	"context"
	"net/url"
	"strconv"

	"json-inspector/internal/domain"
	"json-inspector/internal/har"
)

// ExportHAR writes the captured traffic of one tab as a HAR document, and answers with a name a
// save dialog can open on.
//
// The tab is named the way the window names it — its id, or the page's address when the record has
// no id — and the two sides have to agree on that: this is the second half of
// `frontend/src/lib/recordTabs.ts`'s `tabKeyOf`, and the day one of them changes the other has to.
func (u *UseCase) ExportHAR(ctx context.Context, tabKey string) ([]byte, string, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, "", err
	}

	// The store's own read and not List: the list is capped at what the panel draws, and an export
	// that quietly left out the two-hundred-and-first request would be a file that lies about the
	// session it claims to be.
	records, err := u.store.Records(ctx, workspace, domain.SourceBrowser, 0)
	if err != nil {
		return nil, "", err
	}

	// Named by the first record of the tab, and by a flag rather than by the default still standing:
	// a tab whose name is literally "session" would otherwise rename the file on every row.
	name, named := "session", false
	entries := []har.Entry{}
	for _, rec := range records {
		if tabKeyOf(rec) != tabKey {
			continue
		}
		if !named {
			name, named = tabNameOf(rec), true
		}
		entries = append(entries, har.Entry{
			Record:       rec,
			RequestBody:  u.body(ctx, rec, domain.SideRequest),
			ResponseBody: u.body(ctx, rec, domain.SideResponse),
		})
	}

	// Oldest first: a HAR file is a session in the order it happened, and the list it was built from
	// is newest-first because that is the order a panel shows.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	data, err := har.Export(u.build.Name, u.build.Version, entries)
	if err != nil {
		return nil, "", err
	}
	return data, name, nil
}

// A body that cannot be read is no body: the record is what the file is about, and losing the
// answer to a large one is not a reason to write no file at all.
func (u *UseCase) body(ctx context.Context, rec domain.Record, side domain.BodySide) string {
	text, err := u.store.ReadBody(ctx, rec.ID, side)
	if err != nil {
		return ""
	}
	return text
}

func tabKeyOf(rec domain.Record) string {
	if rec.TabID != 0 {
		return strconv.Itoa(rec.TabID)
	}
	if rec.TabURL != "" {
		return rec.TabURL
	}
	return "unknown"
}

// What the file is called in a save dialog: the page's host when the record knows it, and its title
// otherwise. A name nobody can read is a name nobody can find again.
func tabNameOf(rec domain.Record) string {
	if host := hostOf(rec.TabURL); host != "" {
		return host
	}
	if rec.TabTitle != "" {
		return rec.TabTitle
	}
	return "session"
}

func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
