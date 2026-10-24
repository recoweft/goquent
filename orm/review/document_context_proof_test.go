package review

import (
	"bytes"
	"testing"
)

// This finite model is NOT a certificate producer for Go/package loading. Its
// explicit input universe lets tests falsify the proposed comparison rule.
// No production caller can obtain a narrower context from these test types.
type documentModel struct {
	tree, captured sourceTree
	roles          map[string]string
	external       sourceTree
	environment    string
	identity       string
	priorCount     int
	native         bool
	closed         bool
}

func proveDocumentModel(base, head documentModel, patch []byte) bool {
	if !base.closed || !head.closed || !base.native || !head.native ||
		base.environment == "" || base.environment != head.environment ||
		base.identity != "static-select" || head.identity != base.identity ||
		base.priorCount != 1 || head.priorCount != 1 ||
		!equalTrees(base.external, head.external) || !parseDiff(patch, base.tree, head.tree).valid {
		return false
	}
	// The model has a closed inventory, including the negative dependency set.
	// A real loader has no equivalent closure witness today. An absent capture
	// alone must never turn an arbitrary file into an unconsumed document.
	for _, m := range []documentModel{base, head} {
		if len(m.tree) != len(m.roles) || len(m.tree) > maxTypeFiles {
			return false
		}
		for p, data := range m.tree {
			if !exactPath(p) || len(data) > maxTypeFileBytes {
				return false
			}
			switch m.roles[p] {
			case "document":
				if _, consumed := m.captured[p]; consumed {
					return false
				}
			case "source", "helper", "config", "manifest", "build", "dependency", "consumed-document":
				captured, ok := m.captured[p]
				if !ok || !bytes.Equal(data, captured) {
					return false
				}
			default:
				return false
			}
		}
		for p := range m.captured {
			if _, ok := m.tree[p]; !ok {
				return false
			}
		}
	}
	if len(base.tree) != len(head.tree) || !equalTrees(base.captured, head.captured) {
		return false
	}
	for p, old := range base.tree {
		now, ok := head.tree[p]
		if !ok || base.roles[p] != head.roles[p] ||
			(!bytes.Equal(old, now) && head.roles[p] != "document") {
			return false
		}
	}
	return true
}

func newDocumentModel() documentModel {
	m := documentModel{tree: sourceTree{}, captured: sourceTree{}, roles: map[string]string{},
		external: sourceTree{"toolchain": []byte("fixed\n")}, environment: "fixed-variant", identity: "static-select", priorCount: 1, native: true, closed: true}
	for p, role := range map[string]string{"query.go": "source", "helper.go": "helper", "config.md": "config", "manifest.md": "manifest", "go.mod": "dependency", "build.flags": "build", "notes.data": "document"} {
		m.tree[p] = []byte("before\n")
		m.roles[p] = role
		if role != "document" {
			m.captured[p] = []byte("before\n")
		}
	}
	return m
}

func TestDocumentContextFiniteModel(t *testing.T) {
	cases := []struct {
		name string
		edit func(*documentModel, *documentModel)
		want bool
	}{
		{"listed_unconsumed_document", func(_, _ *documentModel) {}, true},
		{"consumed_document", func(b, h *documentModel) {
			for _, m := range []*documentModel{b, h} {
				m.roles["notes.data"] = "consumed-document"
				m.captured["notes.data"] = m.tree["notes.data"]
			}
		}, false},
		{"ambiguous", func(_, h *documentModel) { h.priorCount = 2 }, false},
		{"unsupported", func(_, h *documentModel) { h.identity = "unsupported" }, false},
		{"dynamic", func(_, h *documentModel) { h.identity = "" }, false},
		{"closure_unavailable", func(_, h *documentModel) { h.closed = false }, false},
		{"custom_driver", func(_, h *documentModel) { h.native = false }, false},
		{"environment", func(_, h *documentModel) { h.environment = "changed" }, false},
		{"capture_mutation", func(_, h *documentModel) { h.captured["query.go"] = []byte("mutated\n") }, false},
		{"external_consumed", func(_, h *documentModel) { h.external["toolchain"] = []byte("changed\n") }, false},
		{"unclassified", func(_, h *documentModel) { delete(h.roles, "notes.data") }, false},
		{"unlisted", func(_, h *documentModel) { h.tree["query.go"] = []byte("unlisted\n") }, false},
	}
	for _, path := range []string{"query.go", "helper.go", "config.md", "manifest.md", "go.mod", "build.flags"} {
		cases = append(cases, struct {
			name string
			edit func(*documentModel, *documentModel)
			want bool
		}{path, func(_, h *documentModel) {
			h.tree[path] = []byte("after\n")
			h.captured[path] = h.tree[path]
		}, false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, head := newDocumentModel(), newDocumentModel()
			head.tree["notes.data"] = []byte("after\n")
			tc.edit(&base, &head)
			old, now := map[string]string{}, map[string]string{}
			for p, b := range base.tree {
				old[p] = string(b)
			}
			for p, b := range head.tree {
				now[p] = string(b)
			}
			if tc.name == "unlisted" {
				now["query.go"] = old["query.go"]
			}
			if got := proveDocumentModel(base, head, changedFixturePatch(old, now)); got != tc.want {
				t.Fatal("finite proof result differs")
			}
		})
	}
}
