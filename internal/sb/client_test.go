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

func TestCreateAndUpdateStory(t *testing.T) {
	c := New("token")
	calls := 0
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("Authorization") != "token" {
			t.Fatalf("missing token header")
		}
		switch req.Method {
		case http.MethodPost:
			if !strings.Contains(req.URL.Path, "/stories") {
				t.Fatalf("unexpected POST url %s", req.URL.Path)
			}
			body := `{"story":{"id":1,"name":"a"}}`
			res := &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
			return res, nil
		case http.MethodPut:
			if !strings.Contains(req.URL.Path, "/stories/1") {
				t.Fatalf("unexpected PUT url %s", req.URL.Path)
			}
			body := `{"story":{"id":1,"name":"b"}}`
			res := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
			return res, nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
		return nil, nil
	})}

	st, err := c.CreateStory(context.Background(), 1, Story{Name: "a"})
	if err != nil || st.Name != "a" {
		t.Fatalf("CreateStory failed: %v, %+v", err, st)
	}
	st2, err := c.UpdateStory(context.Background(), 1, Story{ID: 1, Name: "b"})
	if err != nil || st2.Name != "b" {
		t.Fatalf("UpdateStory failed: %v, %+v", err, st2)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
