package sb

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestListSpaces(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "token" {
			t.Fatalf("unexpected token header: %s", req.Header.Get("Authorization"))
		}
		body := `{"spaces":[{"id":1,"name":"one"}]}`
		res := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}
		return res, nil
	})}
	spaces, err := c.ListSpaces(context.Background())
	if err != nil {
		t.Fatalf("ListSpaces returned error: %v", err)
	}
	if len(spaces) != 1 || spaces[0].ID != 1 || spaces[0].Name != "one" {
		t.Fatalf("unexpected spaces: %+v", spaces)
	}
}

func TestListSpacesNoToken(t *testing.T) {
	c := New("")
	if _, err := c.ListSpaces(context.Background()); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestListStoriesPagination(t *testing.T) {
	c := New("token")
	calls := 0
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		q := req.URL.Query()
		page := q.Get("page")
		var body string
		switch page {
		case "1":
			body = `{"stories":[{"id":1,"name":"a"},{"id":2,"name":"b"}]}`
		case "2":
			body = `{"stories":[{"id":3,"name":"c"}]}`
		default:
			t.Fatalf("unexpected page %s", page)
		}
		res := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}
		return res, nil
	})}
	stories, err := c.ListStories(context.Background(), ListStoriesOpts{SpaceID: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("ListStories returned error: %v", err)
	}
	if len(stories) != 3 {
		t.Fatalf("expected 3 stories, got %d", len(stories))
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestGetStoryBySlug(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		q := req.URL.Query().Get("with_slug")
		if q != "app/de" {
			t.Fatalf("unexpected with_slug %s", q)
		}
		body := `{"story":{"id":2,"name":"de"}}`
		res := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
		return res, nil
	})}
	st, err := c.GetStoryBySlug(context.Background(), 1, "app/de")
	if err != nil {
		t.Fatalf("GetStoryBySlug returned error: %v", err)
	}
	if st.ID != 2 || st.Name != "de" {
		t.Fatalf("unexpected story: %+v", st)
	}
}

func TestGetStoryBySlugNotFound(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		res := &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}
		return res, nil
	})}
	st, err := c.GetStoryBySlug(context.Background(), 1, "missing")
	if err != nil {
		t.Fatalf("GetStoryBySlug returned error: %v", err)
	}
	if st.ID != 0 {
		t.Fatalf("expected zero story, got %+v", st)
	}
}
