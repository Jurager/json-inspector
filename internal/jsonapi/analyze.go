package jsonapi

// Analysis is the result of analyzing a response body: whether it is JSON:API
// and, if so, the object graph used by the map view.
type Analysis struct {
	IsJSONAPI bool   `json:"isJsonAPI"`
	Graph     *Graph `json:"graph,omitempty"`
}

// Analyze inspects raw and, when it looks like JSON:API, builds the object
// graph. For non-JSON:API bodies only IsJSONAPI is set.
func Analyze(raw []byte) *Analysis {
	a := &Analysis{IsJSONAPI: IsJSONAPI(raw)}
	if !a.IsJSONAPI {
		return a
	}
	if doc, err := Parse(raw); err == nil {
		a.Graph = BuildGraph(doc)
	}
	return a
}
