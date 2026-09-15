package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/infra/httpx"
	"json-inspector/internal/usecase/record"
	"json-inspector/internal/usecase/settings"
)

// Differ on purpose: the records feature must not know how a request leaves the process.
type engineExecutor struct {
	engine *httpx.Engine
}

var _ record.Executor = engineExecutor{}

func (e engineExecutor) Execute(ctx context.Context, req record.Request) (*domain.Response, error) {
	return e.engine.Do(ctx, httpx.Spec{
		ID:      req.ID,
		Method:  req.Method,
		URL:     req.URL,
		Headers: req.Headers,
		Body:    req.Body,
		Digest:  req.Digest,
	}), nil
}

func (e engineExecutor) Cancel(id string) bool {
	return e.engine.Cancel(id)
}

func settingsRetention(uc *settings.UseCase) record.RetentionSource {
	return record.RetentionSourceFunc(func(ctx context.Context) (domain.Retention, error) {
		current, err := uc.Snapshot(ctx)
		if err != nil {
			return "", err
		}
		return current.HistoryRetention, nil
	})
}
