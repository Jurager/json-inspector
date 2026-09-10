package jsonapi

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Graph is the "карта объекта": a directed graph where nodes are resources
// (from data and included) and edges are relationships. Resources referenced
// by a relationship but not present in the document are emitted as "missing"
// nodes, so the whole object model is visible.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode is a resource (or a reference to one not present in the document).
type GraphNode struct {
	Key     string `json:"key"`               // "type/id"
	Type    string `json:"type"`              // empty for external-only references is still set from the identifier
	ID      string `json:"id"`
	Label   string `json:"label"`             // short human-readable label
	Source  string `json:"source"`            // "data" | "included" | "missing"
	Related string `json:"related,omitempty"` // related URL, when the resource is missing but has one
}

// GraphEdge is a directed relationship from one resource to another.
type GraphEdge struct {
	From     string `json:"from"` // source resource key
	To       string `json:"to"`   // target resource key
	Rel      string `json:"rel"`  // relationship name
	External bool   `json:"external,omitempty"`
}

// BuildGraph builds the object graph for a document. Output is deterministic.
func BuildGraph(doc *Document) *Graph {
	idx := BuildIndex(doc)

	type nodeRec struct {
		node   GraphNode
		source string
	}
	byKey := map[string]nodeRec{}

	add := func(r Resource, source string) {
		if r.Type == "" || r.ID == "" {
			return
		}
		k := Key(r.Type, r.ID)
		if _, ok := byKey[k]; !ok {
			byKey[k] = nodeRec{GraphNode{
				Key:    k,
				Type:   r.Type,
				ID:     r.ID,
				Label:  resourceLabel(r),
				Source: source,
			}, source}
		}
	}

	dataRes := DataResources(doc)
	for _, r := range dataRes {
		add(r, "data")
	}
	for _, r := range doc.Included {
		add(r, "included")
	}

	// Ensure every resource referenced by a relationship has a node, marking
	// those absent from the document as "missing".
	all := append(append([]Resource{}, dataRes...), doc.Included...)
	var missing []GraphNode
	missingSeen := map[string]bool{}
	for _, r := range all {
		if r.Type == "" || r.ID == "" {
			continue
		}
		for _, rel := range r.Relationships {
			for _, ri := range relIdentifiers(rel.Data) {
				if ri.Type == "" || ri.ID == "" {
					continue
				}
				k := Key(ri.Type, ri.ID)
				if _, ok := idx[k]; ok {
					continue
				}
				if missingSeen[k] {
					continue
				}
				missingSeen[k] = true
				missing = append(missing, GraphNode{
					Key:     k,
					Type:    ri.Type,
					ID:      ri.ID,
					Label:   ri.Type + "/" + ri.ID,
					Source:  "missing",
					Related: rel.Links.Href("related"),
				})
			}
		}
	}

	g := &Graph{Nodes: []GraphNode{}, Edges: []GraphEdge{}}

	// Deterministic node ordering: data, then included, then missing; each by key.
	sort.Slice(missing, func(i, j int) bool { return missing[i].Key < missing[j].Key })
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := byKey[keys[i]], byKey[keys[j]]
		ra, rb := sourceRank(a.source), sourceRank(b.source)
		if ra != rb {
			return ra < rb
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		g.Nodes = append(g.Nodes, byKey[k].node)
	}
	for _, m := range missing {
		g.Nodes = append(g.Nodes, m)
	}

	// Build edges deterministically: resources in document order, relationship
	// names sorted, identifiers in document order.
	for _, r := range all {
		if r.Type == "" || r.ID == "" {
			continue
		}
		from := Key(r.Type, r.ID)
		names := make([]string, 0, len(r.Relationships))
		for name := range r.Relationships {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			rel := r.Relationships[name]
			relURL := rel.Links.Href("related")
			for _, ri := range relIdentifiers(rel.Data) {
				if ri.Type == "" || ri.ID == "" {
					continue
				}
				to := Key(ri.Type, ri.ID)
				_, present := idx[to]
				g.Edges = append(g.Edges, GraphEdge{
					From:     from,
					To:       to,
					Rel:      name,
					External: !present,
				})
				if !present && relURL != "" {
					// attach related URL to the missing node if not already set
					for i := range g.Nodes {
						if g.Nodes[i].Key == to && g.Nodes[i].Source == "missing" && g.Nodes[i].Related == "" {
							g.Nodes[i].Related = relURL
						}
					}
				}
			}
		}
	}
	sort.Slice(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Rel != b.Rel {
			return a.Rel < b.Rel
		}
		return a.To < b.To
	})

	return g
}

func sourceRank(s string) int {
	switch s {
	case "data":
		return 0
	case "included":
		return 1
	default:
		return 2
	}
}

// relIdentifiers extracts resource identifiers from relationship data, which
// may be a single object, an array of objects, or null.
func relIdentifiers(raw json.RawMessage) []ResourceIdentifier {
	d := bytes.TrimSpace(raw)
	if len(d) == 0 || bytes.Equal(d, []byte("null")) {
		return nil
	}
	switch d[0] {
	case '{':
		var ri ResourceIdentifier
		if err := json.Unmarshal(d, &ri); err != nil {
			return nil
		}
		return []ResourceIdentifier{ri}
	case '[':
		var ris []ResourceIdentifier
		if err := json.Unmarshal(d, &ris); err != nil {
			return nil
		}
		return ris
	default:
		return nil
	}
}

// resourceLabel picks a short human-readable label for a resource from its
// attributes, preferring common "name-like" fields.
func resourceLabel(r Resource) string {
	if len(r.Attributes) == 0 {
		return r.Type + "/" + r.ID
	}
	var attrs map[string]any
	if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
		return r.Type + "/" + r.ID
	}
	for _, key := range []string{"title", "name", "label", "username", "email", "slug", "code", "key"} {
		if v, ok := attrs[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	for _, v := range attrs {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return r.Type + "/" + r.ID
}
