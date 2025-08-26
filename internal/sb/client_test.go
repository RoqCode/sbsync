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
		want    *float64 // Use pointer to distinguish between 1, 0, and nil (omitted)
	}{
		{true, func() *float64 { v := float64(1); return &v }()},
		{false, nil}, // When publish=false, field should be omitted
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
			if tt.want == nil {
				// Expect publish field to be omitted
				if ok {
					t.Fatalf("expected publish field to be omitted, but found: %v", val)
				}
			} else {
				// Expect publish field to be present with specific value
				if !ok {
					t.Fatalf("publish field missing or not number: %#v", payload["publish"])
				}
				if val != *tt.want {
					t.Fatalf("expected publish %v, got %v", *tt.want, val)
				}
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
		want    *float64 // Use pointer to distinguish between 1, 0, and nil (omitted)
	}{
		{true, func() *float64 { v := float64(1); return &v }()},
		{false, nil}, // When publish=false, field should be omitted
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
			if tt.want == nil {
				// Expect publish field to be omitted
				if ok {
					t.Fatalf("expected publish field to be omitted, but found: %v", val)
				}
			} else {
				// Expect publish field to be present with specific value
				if !ok {
					t.Fatalf("publish field missing or not number: %#v", payload["publish"])
				}
				if val != *tt.want {
					t.Fatalf("expected publish %v, got %v", *tt.want, val)
				}
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

func TestUpdateStoryUUID(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "PUT" {
			t.Fatalf("expected PUT method, got %s", req.Method)
		}
		expectedPath := "/v1/spaces/1/stories/2/update_uuid"
		if !strings.HasSuffix(req.URL.Path, expectedPath) {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload map[string]string
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if payload["uuid"] != "new-uuid" {
			t.Fatalf("expected uuid new-uuid, got %s", payload["uuid"])
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	})}
	if err := c.UpdateStoryUUID(context.Background(), 1, 2, "new-uuid"); err != nil {
		t.Fatalf("UpdateStoryUUID returned error: %v", err)
	}
}

func TestUpdateStoryUUIDNoToken(t *testing.T) {
	c := New("")
	if err := c.UpdateStoryUUID(context.Background(), 1, 2, "uuid"); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestGetStoryWithContentAndGetStory(t *testing.T) {
	c := New("token")
	calls := 0
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != "GET" {
			t.Fatalf("expected GET method, got %s", req.Method)
		}
		if req.Header.Get("Authorization") != "token" {
			t.Fatalf("unexpected token header: %s", req.Header.Get("Authorization"))
		}
		expectedPath := "/v1/spaces/1/stories/2"
		if req.URL.Path != expectedPath {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"story":{"id":2,"name":"story","content":{"body":{}}}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	st, err := c.GetStoryWithContent(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("GetStoryWithContent returned error: %v", err)
	}
	if st.ID != 2 || st.Content == nil {
		t.Fatalf("unexpected story: %+v", st)
	}
	st2, err := c.GetStory(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("GetStory returned error: %v", err)
	}
	if st2.ID != 2 {
		t.Fatalf("unexpected story from GetStory: %+v", st2)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestGetStoryWithContentNoToken(t *testing.T) {
	c := New("")
	if _, err := c.GetStoryWithContent(context.Background(), 1, 2); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestGetStoriesBySlug(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("with_slug") != "news" {
			t.Fatalf("unexpected with_slug: %s", req.URL.Query().Get("with_slug"))
		}
		body := `{"stories":[{"id":1},{"id":2}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	stories, err := c.GetStoriesBySlug(context.Background(), 1, "news")
	if err != nil {
		t.Fatalf("GetStoriesBySlug returned error: %v", err)
	}
	if len(stories) != 2 {
		t.Fatalf("expected 2 stories, got %d", len(stories))
	}
}

func TestGetStoriesBySlugNoToken(t *testing.T) {
	c := New("")
	if _, err := c.GetStoriesBySlug(context.Background(), 1, "slug"); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestCreateStory(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "POST" {
			t.Fatalf("expected POST method, got %s", req.Method)
		}
		expectedPath := "/v1/spaces/1/stories"
		if req.URL.Path != expectedPath {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload storyResp
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if payload.Story.Name != "n" {
			t.Fatalf("unexpected story payload: %+v", payload.Story)
		}
		resBody := `{"story":{"id":5,"name":"n"}}`
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(resBody)), Header: make(http.Header)}, nil
	})}
	st, err := c.CreateStory(context.Background(), 1, Story{Name: "n"})
	if err != nil {
		t.Fatalf("CreateStory returned error: %v", err)
	}
	if st.ID != 5 {
		t.Fatalf("unexpected story: %+v", st)
	}
}

func TestCreateStoryNoToken(t *testing.T) {
	c := New("")
	if _, err := c.CreateStory(context.Background(), 1, Story{}); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestGetStoryWithVersion(t *testing.T) {
	tests := []struct{ version string }{
		{""},
		{"draft"},
	}
	for _, tt := range tests {
		c := New("token")
		c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			if q.Get("resolve_relations") != "1" {
				t.Fatalf("resolve_relations missing: %v", q)
			}
			if tt.version == "" {
				if q.Get("version") != "" {
					t.Fatalf("unexpected version %q", q.Get("version"))
				}
			} else {
				if q.Get("version") != tt.version {
					t.Fatalf("version = %q, want %q", q.Get("version"), tt.version)
				}
			}
			body := `{"story":{"id":2}}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
		st, err := c.getStoryWithVersion(context.Background(), 1, 2, tt.version)
		if err != nil {
			t.Fatalf("getStoryWithVersion returned error: %v", err)
		}
		if st.ID != 2 {
			t.Fatalf("unexpected story: %+v", st)
		}
	}
}
