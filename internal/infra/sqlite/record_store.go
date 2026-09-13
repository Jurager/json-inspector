package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"json-inspector/internal/domain"
)

// SaveRecord writes a record and both of its bodies in one transaction: a row whose body is missing
// would open as an empty document, and a body with no row would be unreachable.
func (s *Store) SaveRecord(ctx context.Context, rec domain.Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("saving record %s: %w", rec.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	requestHeaders, err := json.Marshal(orEmptyPairs(rec.RequestHeaders))
	if err != nil {
		return fmt.Errorf("saving record %s: %w", rec.ID, err)
	}
	responseHeaders, err := json.Marshal(orEmptyPairs(rec.ResponseHeaders))
	if err != nil {
		return fmt.Errorf("saving record %s: %w", rec.ID, err)
	}
	cookies, err := json.Marshal(orEmptyCookies(rec.RequestCookies))
	if err != nil {
		return fmt.Errorf("saving record %s: %w", rec.ID, err)
	}

	finishedAt := time.Now().UnixMilli()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO records (id, source, method, url, status, status_text, content_type, error,
		                      cancelled, duration_us, dns_us, connect_us, tls_us, wait_us, download_us,
		                      request_bytes, response_bytes, request_headers_json,
		                      response_headers_json, request_cookies_json, started_at, finished_at,
		                      tab_id, tab_title, tab_url, favicon_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, string(rec.Source), rec.Method, rec.URL, rec.Status, rec.StatusText, rec.ContentType,
		rec.Error, boolToInt(rec.Cancelled), rec.DurationUs, rec.DNSUs, rec.ConnectUs, rec.TLSUs,
		rec.WaitUs, rec.DownloadUs, rec.RequestBytes, rec.ResponseBytes,
		string(requestHeaders), string(responseHeaders), string(cookies), rec.StartedAt, finishedAt,
		nullIfZero(rec.TabID), nullIfEmpty(rec.TabTitle), nullIfEmpty(rec.TabURL), nullIfEmpty(rec.FavIconURL))
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("record %s: %w", rec.ID, domain.ErrConflict)
		}
		return fmt.Errorf("saving record %s: %w", rec.ID, err)
	}

	seq, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("saving record %s: %w", rec.ID, err)
	}
	for _, side := range []struct {
		kind domain.BodySide
		body *domain.BodyRef
	}{
		{domain.SideRequest, rec.RequestBody},
		{domain.SideResponse, rec.ResponseBody},
	} {
		if side.body == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO record_bodies (record_seq, side, content, encoding, size, truncated)
			 VALUES (?, ?, ?, 'utf8', ?, ?)`,
			seq, string(side.kind), []byte(side.body.Inline), side.body.Size, boolToInt(side.body.Truncated)); err != nil {
			return fmt.Errorf("saving the %s body of %s: %w", side.kind, rec.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("saving record %s: %w", rec.ID, err)
	}
	return nil
}

// Records lists the newest records of one source, or of both when source is empty. Everything but
// the bodies travels — the list is what the pane draws, and a moment later it is drawn for a record
// the window has not selected yet, so a second call would be a second wait. Each body comes as its
// size alone: the viewer knows what it is asking for before it asks.
func (s *Store) Records(ctx context.Context, source domain.RecordSource, limit int) ([]domain.Record, error) {
	scope, args := listScope(source, limit)

	rows, err := s.db.QueryContext(ctx,
		`SELECT seq, id, source, method, url, status, status_text, content_type, error, cancelled,
		        duration_us, dns_us, connect_us, tls_us, wait_us, download_us,
		        request_bytes, response_bytes, request_headers_json, response_headers_json,
		        request_cookies_json, started_at, ifnull(tab_id, 0), ifnull(tab_title, ''),
		        ifnull(tab_url, ''), ifnull(favicon_url, '')
		   FROM records`+scope, args...)
	if err != nil {
		return nil, fmt.Errorf("listing records: %w", err)
	}
	defer rows.Close()

	out := []domain.Record{}
	seqs := []int64{}
	for rows.Next() {
		var (
			rec                                      domain.Record
			seq                                      int64
			cancelled                                int
			requestHeaders, responseHeaders, cookies string
		)
		// The phases come back as nullable columns: absent is a phase that did not happen, which is
		// not the same thing as one that took no time.
		if err := rows.Scan(&seq, &rec.ID, &rec.Source, &rec.Method, &rec.URL, &rec.Status, &rec.StatusText,
			&rec.ContentType, &rec.Error, &cancelled, &rec.DurationUs, &rec.DNSUs, &rec.ConnectUs, &rec.TLSUs,
			&rec.WaitUs, &rec.DownloadUs, &rec.RequestBytes, &rec.ResponseBytes, &requestHeaders,
			&responseHeaders, &cookies, &rec.StartedAt, &rec.TabID, &rec.TabTitle, &rec.TabURL,
			&rec.FavIconURL); err != nil {
			return nil, fmt.Errorf("listing records: %w", err)
		}
		rec.Cancelled = cancelled != 0
		if err := json.Unmarshal([]byte(requestHeaders), &rec.RequestHeaders); err != nil {
			return nil, fmt.Errorf("reading the request headers of %s: %w", rec.ID, err)
		}
		if err := json.Unmarshal([]byte(responseHeaders), &rec.ResponseHeaders); err != nil {
			return nil, fmt.Errorf("reading the response headers of %s: %w", rec.ID, err)
		}
		if err := json.Unmarshal([]byte(cookies), &rec.RequestCookies); err != nil {
			return nil, fmt.Errorf("reading the cookies of %s: %w", rec.ID, err)
		}
		out = append(out, rec)
		seqs = append(seqs, seq)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing records: %w", err)
	}

	if err := s.attachBodyRefs(ctx, out, seqs, scope, args); err != nil {
		return nil, err
	}
	return out, nil
}

// Record reads one record by id, with its body references and nothing in them: a run's row opens the
// record it produced, and the window asks for the bodies only once the viewer is on screen.
func (s *Store) Record(ctx context.Context, id string) (domain.Record, error) {
	var (
		rec                                      domain.Record
		seq                                      int64
		cancelled                                int
		requestHeaders, responseHeaders, cookies string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT seq, id, source, method, url, status, status_text, content_type, error, cancelled,
		        duration_us, dns_us, connect_us, tls_us, wait_us, download_us,
		        request_bytes, response_bytes, request_headers_json, response_headers_json,
		        request_cookies_json, started_at, ifnull(tab_id, 0), ifnull(tab_title, ''),
		        ifnull(tab_url, ''), ifnull(favicon_url, '')
		   FROM records WHERE id = ?`, id).
		Scan(&seq, &rec.ID, &rec.Source, &rec.Method, &rec.URL, &rec.Status, &rec.StatusText,
			&rec.ContentType, &rec.Error, &cancelled, &rec.DurationUs, &rec.DNSUs, &rec.ConnectUs,
			&rec.TLSUs, &rec.WaitUs, &rec.DownloadUs, &rec.RequestBytes, &rec.ResponseBytes,
			&requestHeaders, &responseHeaders, &cookies, &rec.StartedAt, &rec.TabID, &rec.TabTitle,
			&rec.TabURL, &rec.FavIconURL)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Record{}, fmt.Errorf("record %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Record{}, fmt.Errorf("reading record %s: %w", id, err)
	}
	rec.Cancelled = cancelled != 0
	for _, part := range []struct {
		raw  string
		into any
		what string
	}{
		{requestHeaders, &rec.RequestHeaders, "request headers"},
		{responseHeaders, &rec.ResponseHeaders, "response headers"},
		{cookies, &rec.RequestCookies, "cookies"},
	} {
		if err := json.Unmarshal([]byte(part.raw), part.into); err != nil {
			return domain.Record{}, fmt.Errorf("reading the %s of %s: %w", part.what, id, err)
		}
	}
	// The same call the list makes, for a list of one: it fills the record in place, which is where the
	// body references come from.
	records := []domain.Record{rec}
	if err := s.attachBodyRefs(ctx, records, []int64{seq}, ` WHERE id = ?`, []any{id}); err != nil {
		return domain.Record{}, err
	}
	return records[0], nil
}

// attachBodyRefs says which bodies the listed records have and how big they are, without reading
// any of them. It is a second query rather than a join because a record has two bodies: a join
// would repeat every row of the list, and a LIMIT over it would count bodies instead of records.
func (s *Store) attachBodyRefs(ctx context.Context, records []domain.Record, seqs []int64, scope string, args []any) error {
	if len(seqs) == 0 {
		return nil
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT record_seq, side, size, truncated
		   FROM record_bodies
		  WHERE record_seq IN (SELECT seq FROM records`+scope+`)`, args...)
	if err != nil {
		return fmt.Errorf("listing the bodies of records: %w", err)
	}
	defer rows.Close()

	at := make(map[int64]*domain.Record, len(seqs))
	for i, seq := range seqs {
		at[seq] = &records[i]
	}

	for rows.Next() {
		var (
			seq       int64
			side      domain.BodySide
			size      int64
			truncated int
		)
		if err := rows.Scan(&seq, &side, &size, &truncated); err != nil {
			return fmt.Errorf("listing the bodies of records: %w", err)
		}
		rec, ok := at[seq]
		if !ok {
			continue
		}
		ref := &domain.BodyRef{Size: size, Truncated: truncated != 0}
		if side == domain.SideRequest {
			rec.RequestBody = ref
		} else {
			rec.ResponseBody = ref
		}
	}
	return rows.Err()
}

// listScope is the one definition of "the records the list shows". Both queries above have to agree
// on it, or a body would arrive for a record that was never listed.
func listScope(source domain.RecordSource, limit int) (string, []any) {
	scope := ""
	args := []any{}
	if source != "" {
		scope = ` WHERE source = ?`
		args = append(args, string(source))
	}
	// The sequence breaks ties: a burst of captures shares a millisecond, and history must not
	// reshuffle between two reads of it.
	scope += ` ORDER BY started_at DESC, seq DESC`
	if limit > 0 {
		scope += ` LIMIT ?`
		args = append(args, limit)
	}
	return scope, args
}

// ReadBody is the call a viewer makes for a body that did not travel with the record.
func (s *Store) ReadBody(ctx context.Context, id string, side domain.BodySide) (string, error) {
	var content []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM record_bodies
		  WHERE record_seq = (SELECT seq FROM records WHERE id = ?) AND side = ?`,
		id, string(side)).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("the %s body of %s: %w", side, id, domain.ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("reading the %s body of %s: %w", side, id, err)
	}
	return string(content), nil
}

// DeleteRecords removes records by id; their bodies go with them through the foreign key.
func (s *Store) DeleteRecords(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("deleting records: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM records WHERE id = ?`, id); err != nil {
			return fmt.Errorf("deleting record %s: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("deleting records: %w", err)
	}
	return nil
}

// Prune drops what the retention rules no longer keep and reports how many rows went. SQLite's
// `LIMIT -1 OFFSET n` is the way to say "everything after the newest n".
func (s *Store) Prune(ctx context.Context, opts domain.PruneOptions) (int, error) {
	total := 0

	if opts.MaxAge > 0 {
		cutoff := time.Now().Add(-opts.MaxAge).UnixMilli()
		res, err := s.db.ExecContext(ctx, `DELETE FROM records WHERE started_at < ?`, cutoff)
		if err != nil {
			return total, fmt.Errorf("pruning records older than %s: %w", opts.MaxAge, err)
		}
		count, _ := res.RowsAffected()
		total += int(count)
	}

	if opts.MaxCount > 0 {
		res, err := s.db.ExecContext(ctx,
			`DELETE FROM records WHERE seq IN (
			   SELECT seq FROM records ORDER BY started_at DESC, seq DESC LIMIT -1 OFFSET ?)`,
			opts.MaxCount)
		if err != nil {
			return total, fmt.Errorf("pruning records beyond %d: %w", opts.MaxCount, err)
		}
		count, _ := res.RowsAffected()
		total += int(count)
	}

	if total > 0 {
		// Space comes back only in chunks, and only because auto_vacuum is incremental: a full
		// VACUUM here would rewrite the whole file on every prune.
		if _, err := s.db.ExecContext(ctx, `PRAGMA incremental_vacuum(256)`); err != nil {
			return total, fmt.Errorf("reclaiming space: %w", err)
		}
	}
	return total, nil
}

func orEmptyPairs(pairs []domain.HeaderPair) []domain.HeaderPair {
	if pairs == nil {
		return []domain.HeaderPair{}
	}
	return pairs
}

func orEmptyCookies(cookies []domain.CookieRow) []domain.CookieRow {
	if cookies == nil {
		return []domain.CookieRow{}
	}
	return cookies
}

func nullIfZero(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
