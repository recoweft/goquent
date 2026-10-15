package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/review"
)

func TestReviewV2CoveragePrivacy(t *testing.T) {
	s := NewServer(Options{})
	result, e := s.CallTool(context.Background(), "review_query", map[string]any{"sql": "SELECT MCP_PRIVATE_CANARY FROM secrets", "format": "json-v2"})
	if e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(result)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "MCP_PRIVATE_CANARY") {
		t.Fatal("private SQL exposed")
	}
	if len(result.Content) == 0 {
		t.Fatal("missing content")
	}
	v, e := review.DecodeReportViewV2([]byte(result.Content[0].Text))
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Operations) != 1 || v.Operations[0].Detail.Bindings != "unknown" || v.Operations[0].Detail.Policy != "unknown" || v.Operations[0].Correspondence.Source != "unconfirmed" {
		t.Fatal("external SQL promoted")
	}
}
