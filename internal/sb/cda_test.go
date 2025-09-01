package sb

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWalkStoriesByPrefix_Pagination(t *testing.T) {
	c := NewCDA("cdatoken")
	calls := 0
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		// page param
		page := req.URL.Query().Get("page")
		var body string
		switch page {
		case "1":
			body = `{"stories":[{"id":1,"content":{}},{"id":2,"content":{}}],"per_page":2,"page":1}`
		case "2":
			body = `{"stories":[{"id":3,"content":{}}],"per_page":2,"page":2}`
		default:
			t.Fatalf("unexpected page %s", page)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	var ids []int
	err := c.WalkStoriesByPrefix(context.Background(), "root", "published", 2, func(m map[string]any) error {
		if v, ok := m["id"].(float64); ok {
			ids = append(ids, int(v))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkStoriesByPrefix error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("want 3 ids, got %d", len(ids))
	}
}
