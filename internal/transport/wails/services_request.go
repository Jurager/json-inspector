package wails

import (
	"json-inspector/internal/domain"
	"json-inspector/internal/infra/httpx"
)

// RequestsService sends what the request builder produces. It stays this thin on purpose: the
// engine is where a request is actually sent, and the record it produces is written by the records
// feature, not here.
type RequestsService struct {
	engine *httpx.Engine
}

func NewRequestsService(engine *httpx.Engine) *RequestsService {
	return &RequestsService{engine: engine}
}

func (s *RequestsService) SendRequest(method, url string, headers map[string]string, body string) *domain.Response {
	return s.engine.Send(method, url, headers, body)
}

func (s *RequestsService) Fetch(url string, headers map[string]string) *domain.Response {
	return s.engine.Fetch(url, headers)
}

// CancelRequest stops whatever is in flight. It keeps its old name and its old meaning — the
// window has one request at a time and cannot know an id — while the engine itself cancels by id.
func (s *RequestsService) CancelRequest() {
	s.engine.CancelAll()
}
