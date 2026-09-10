package jsonapi

import (
	"strings"
	"testing"
)

// fixture is a compound JSON:API document based on the spec's example.
const fixture = `{
  "jsonapi": {"version": "1.0"},
  "links": {
    "self": "http://example.com/articles",
    "next": "http://example.com/articles?page[offset]=2",
    "last": "http://example.com/articles?page[offset]=10"
  },
  "data": [{
    "type": "articles",
    "id": "1",
    "attributes": {"title": "JSON:API paints my bikeshed!"},
    "relationships": {
      "author": {
        "links": {"self": "http://example.com/articles/1/relationships/author", "related": "http://example.com/articles/1/author"},
        "data": {"type": "people", "id": "9"}
      },
      "comments": {
        "links": {"self": "http://example.com/articles/1/relationships/comments", "related": "http://example.com/articles/1/comments"},
        "data": [{"type": "comments", "id": "5"}, {"type": "comments", "id": "12"}]
      }
    }
  }],
  "included": [
    {
      "type": "people",
      "id": "9",
      "attributes": {"first-name": "Dan", "last-name": "Gebhardt", "twitter": "dgeb"},
      "links": {"self": "http://example.com/people/9"}
    },
    {
      "type": "comments",
      "id": "5",
      "attributes": {"body": "First!"},
      "relationships": {"author": {"data": {"type": "people", "id": "2"}}}
    },
    {
      "type": "comments",
      "id": "12",
      "attributes": {"body": "I like XML better"},
      "relationships": {"author": {"data": {"type": "people", "id": "9"}}}
    }
  ]
}`

func TestIsJSONAPI(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"compound document", fixture, true},
		{"single resource with type/id", `{"data":{"type":"articles","id":"1","attributes":{"title":"x"}}}`, true},
		{"plain json object", `{"foo":1,"bar":"baz"}`, false},
		{"json array", `[1,2,3]`, false},
		{"data without type/id", `{"data":{"name":"x"}}`, false},
		{"null data only", `{"data":null}`, false},
		{"errors only", `{"errors":[{"title":"boom"}]}`, false},
		{"jsonapi member present", `{"jsonapi":{"version":"1.0"},"data":[]}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsJSONAPI([]byte(c.raw)); got != c.want {
				t.Fatalf("IsJSONAPI() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestBuildIndex(t *testing.T) {
	doc, err := Parse([]byte(fixture))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	idx := BuildIndex(doc)
	for _, k := range []string{"articles/1", "people/9", "comments/5", "comments/12"} {
		if _, ok := idx[k]; !ok {
			t.Errorf("index missing key %q", k)
		}
	}
	if _, ok := idx["people/2"]; ok {
		t.Errorf("people/2 should not be in the document")
	}
}

func TestDataResources(t *testing.T) {
	doc, _ := Parse([]byte(fixture))
	rs := DataResources(doc)
	if len(rs) != 1 || rs[0].Type != "articles" || rs[0].ID != "1" {
		t.Fatalf("DataResources = %+v", rs)
	}
}

func TestBuildGraph(t *testing.T) {
	doc, _ := Parse([]byte(fixture))
	g := BuildGraph(doc)

	// Expected nodes: articles/1 (data), people/9 + comments/5 + comments/12
	// (included), people/2 (missing, referenced by comments/5 but not included).
	wantNodes := map[string]string{
		"articles/1": "data",
		"people/9":   "included",
		"comments/5": "included",
		"comments/12": "included",
		"people/2":   "missing",
	}
	if len(g.Nodes) != len(wantNodes) {
		t.Fatalf("got %d nodes, want %d: %+v", len(g.Nodes), len(wantNodes), g.Nodes)
	}
	for _, n := range g.Nodes {
		if want, ok := wantNodes[n.Key]; !ok {
			t.Errorf("unexpected node %q", n.Key)
		} else if n.Source != want {
			t.Errorf("node %q source = %q, want %q", n.Key, n.Source, want)
		}
	}

	// Expected edges.
	wantEdges := map[string]bool{
		"articles/1|author|people/9":      true,
		"articles/1|comments|comments/5":  true,
		"articles/1|comments|comments/12": true,
		"comments/5|author|people/2":      true, // external
		"comments/12|author|people/9":     true,
	}
	if len(g.Edges) != len(wantEdges) {
		t.Fatalf("got %d edges, want %d: %+v", len(g.Edges), len(wantEdges), g.Edges)
	}
	for _, e := range g.Edges {
		k := e.From + "|" + e.Rel + "|" + e.To
		if !wantEdges[k] {
			t.Errorf("unexpected edge %q", k)
		}
		if k == "comments/5|author|people/2" && !e.External {
			t.Errorf("edge comments/5 -> people/2 should be external")
		}
	}

	// The "missing" people/2 node should carry the related URL from articles'
	// author relationship? No — people/2 is referenced by comments/5 which has
	// no related link, so Related must be empty here.
	for _, n := range g.Nodes {
		if n.Key == "people/2" && n.Related != "" {
			t.Errorf("people/2 should have empty Related, got %q", n.Related)
		}
	}

	// Labels come from attributes.
	for _, n := range g.Nodes {
		if n.Key == "articles/1" && n.Label != "JSON:API paints my bikeshed!" {
			t.Errorf("articles/1 label = %q", n.Label)
		}
		if n.Key == "comments/5" && n.Label != "First!" {
			t.Errorf("comments/5 label = %q", n.Label)
		}
	}
}

func TestLinksHref(t *testing.T) {
	raw := `{"links":{"self":"http://x/a","related":{"href":"http://x/b","meta":{"n":1}},"empty":{}}}`
	doc, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := doc.Links.Href("self"); got != "http://x/a" {
		t.Errorf("self = %q", got)
	}
	if got := doc.Links.Href("related"); got != "http://x/b" {
		t.Errorf("related = %q", got)
	}
	if got := doc.Links.Href("empty"); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := doc.Links.Href("nope"); got != "" {
		t.Errorf("nope = %q", got)
	}
}

func TestAnalyzeNonJSONAPI(t *testing.T) {
	a := Analyze([]byte(`{"hello":"world"}`))
	if a.IsJSONAPI {
		t.Fatal("expected IsJSONAPI=false")
	}
	if a.Graph != nil {
		t.Fatal("expected nil graph for non-JSON:API")
	}
}

func TestAnalyzeJSONAPI(t *testing.T) {
	a := Analyze([]byte(fixture))
	if !a.IsJSONAPI {
		t.Fatal("expected IsJSONAPI=true")
	}
	if a.Graph == nil || len(a.Graph.Nodes) == 0 {
		t.Fatal("expected a non-empty graph")
	}
}

func TestResourceLabelFallback(t *testing.T) {
	r := Resource{Type: "things", ID: "42", Attributes: []byte(`{"count":3}`)}
	if got := resourceLabel(r); got != "things/42" {
		t.Errorf("label = %q", got)
	}
	if !strings.Contains(resourceLabel(Resource{Type: "t", ID: "1"}), "t/1") {
		t.Errorf("expected type/id fallback")
	}
}
