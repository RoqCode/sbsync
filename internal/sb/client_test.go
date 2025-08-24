package sb

import (
	"context"
	"encoding/json"
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

func TestPublishFlagNumericUpdateStory(t *testing.T) {
	tests := []struct {
		publish bool
		want    float64
	}{
		{true, 1},
		{false, 0},
	}
	for _, tt := range tests {
		c := New("token")
		c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &payload); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			val, ok := payload["publish"].(float64)
			if !ok {
				t.Fatalf("publish field missing or not number: %#v", payload["publish"])
			}
			if val != tt.want {
				t.Fatalf("expected publish %v, got %v", tt.want, val)
			}
			resBody := `{"story":{"id":1}}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(resBody)), Header: make(http.Header)}, nil
		})}
		_, err := c.UpdateStory(context.Background(), 1, Story{ID: 1}, tt.publish)
		if err != nil {
			t.Fatalf("UpdateStory returned error: %v", err)
		}
	}
}

func TestPublishFlagNumericCreateStory(t *testing.T) {
	tests := []struct {
		publish bool
		want    float64
	}{
		{true, 1},
		{false, 0},
	}
	for _, tt := range tests {
		c := New("token")
		c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &payload); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			val, ok := payload["publish"].(float64)
			if !ok {
				t.Fatalf("publish field missing or not number: %#v", payload["publish"])
			}
			if val != tt.want {
				t.Fatalf("expected publish %v, got %v", tt.want, val)
			}
			resBody := `{"story":{"id":1}}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(resBody)), Header: make(http.Header)}, nil
		})}
		_, err := c.CreateStoryWithPublish(context.Background(), 1, Story{}, tt.publish)
		if err != nil {
			t.Fatalf("CreateStoryWithPublish returned error: %v", err)
		}
	}
}
