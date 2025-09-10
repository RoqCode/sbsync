package sb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestListComponents(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("want GET, got %s", req.Method)
		}
		if req.Header.Get("Authorization") != "token" {
			t.Fatalf("missing token header")
		}
		if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/components") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"components":[{"id":1,"name":"comp","display_name":"Comp","schema":{}}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	comps, err := c.ListComponents(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListComponents error: %v", err)
	}
	if len(comps) != 1 || comps[0].ID != 1 || comps[0].Name != "comp" {
		t.Fatalf("unexpected components: %+v", comps)
	}
}

func TestCreateComponent(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", req.Method)
		}
		if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/components") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		b, _ := io.ReadAll(req.Body)
		var payload struct {
			Component Component `json:"component"`
		}
		if err := json.Unmarshal(b, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if payload.Component.Name != "comp" {
			t.Fatalf("unexpected payload: %+v", payload.Component)
		}
		res := `{"component":{"id":7,"name":"comp"}}`
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(res)), Header: make(http.Header)}, nil
	})}
	got, err := c.CreateComponent(context.Background(), 1, Component{Name: "comp"})
	if err != nil {
		t.Fatalf("CreateComponent error: %v", err)
	}
	if got.ID != 7 {
		t.Fatalf("unexpected component: %+v", got)
	}
}

func TestUpdateComponent(t *testing.T) {
	c := New("token")
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPut {
			t.Fatalf("want PUT, got %s", req.Method)
		}
		if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/components/7") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		b, _ := io.ReadAll(req.Body)
		var payload struct {
			Component Component `json:"component"`
		}
		if err := json.Unmarshal(b, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if payload.Component.ID != 7 {
			t.Fatalf("unexpected id in payload: %+v", payload.Component)
		}
		res := `{"component":{"id":7,"name":"comp2"}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(res)), Header: make(http.Header)}, nil
	})}
	got, err := c.UpdateComponent(context.Background(), 1, Component{ID: 7, Name: "comp2"})
	if err != nil {
		t.Fatalf("UpdateComponent error: %v", err)
	}
	if got.Name != "comp2" {
		t.Fatalf("unexpected component: %+v", got)
	}
}

func TestListComponentGroupsAndCreate(t *testing.T) {
	c := New("token")
	// List
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("want GET, got %s", req.Method)
		}
		if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/component_groups") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"component_groups":[{"uuid":"u1","name":"grp"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	groups, err := c.ListComponentGroups(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListComponentGroups error: %v", err)
	}
	if len(groups) != 1 || groups[0].UUID != "u1" {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	// Create
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", req.Method)
		}
		if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/component_groups") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"component_group":{"uuid":"u2","name":"new"}}`
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	grp, err := c.CreateComponentGroup(context.Background(), 1, "new")
	if err != nil {
		t.Fatalf("CreateComponentGroup error: %v", err)
	}
	if grp.UUID != "u2" || grp.Name != "new" {
		t.Fatalf("unexpected group: %+v", grp)
	}
}

func TestListInternalTagsAndCreate(t *testing.T) {
	c := New("token")
	// List
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("want GET, got %s", req.Method)
		}
		if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/internal_tags") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"internal_tags":[{"id":9,"name":"tag","object_type":"component"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	tags, err := c.ListInternalTags(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListInternalTags error: %v", err)
	}
	if len(tags) != 1 || tags[0].ID != 9 || tags[0].ObjectType != "component" {
		t.Fatalf("unexpected tags: %+v", tags)
	}
	// Create
	c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", req.Method)
		}
		if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/internal_tags") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body := `{"internal_tag":{"id":11,"name":"x","object_type":"component"}}`
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	tag, err := c.CreateInternalTag(context.Background(), 1, "x", "component")
	if err != nil {
		t.Fatalf("CreateInternalTag error: %v", err)
	}
	if tag.ID != 11 || tag.Name != "x" {
		t.Fatalf("unexpected tag: %+v", tag)
	}
}

func TestComponentsNoToken(t *testing.T) {
    c := New("")
    if _, err := c.ListComponents(context.Background(), 1); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.CreateComponent(context.Background(), 1, Component{}); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.UpdateComponent(context.Background(), 1, Component{}); err == nil {
        t.Fatal("expected error for missing id / token")
    }
    if _, err := c.ListComponentGroups(context.Background(), 1); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.CreateComponentGroup(context.Background(), 1, "g"); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.ListInternalTags(context.Background(), 1); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.CreateInternalTag(context.Background(), 1, "t", "component"); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.ListPresets(context.Background(), 1); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.CreatePreset(context.Background(), 1, ComponentPreset{}); err == nil {
        t.Fatal("expected error for empty token")
    }
    if _, err := c.UpdatePreset(context.Background(), 1, ComponentPreset{}); err == nil {
        t.Fatal("expected error for empty token / id")
    }
}

func TestListPresetsAndCreateUpdate(t *testing.T) {
    c := New("token")
    // List
    c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
        if req.Method != http.MethodGet {
            t.Fatalf("want GET, got %s", req.Method)
        }
        if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/presets") {
            t.Fatalf("unexpected path: %s", req.URL.Path)
        }
        body := `{"presets":[{"id":5,"name":"Default","component_id":7,"preset":{},"image":"//img.png"}]}`
        return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
    })}
    presets, err := c.ListPresets(context.Background(), 1)
    if err != nil {
        t.Fatalf("ListPresets error: %v", err)
    }
    if len(presets) != 1 || presets[0].ID != 5 || presets[0].ComponentID != 7 {
        t.Fatalf("unexpected presets: %+v", presets)
    }
    // Create
    c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
        if req.Method != http.MethodPost {
            t.Fatalf("want POST, got %s", req.Method)
        }
        if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/presets") {
            t.Fatalf("unexpected path: %s", req.URL.Path)
        }
        b, _ := io.ReadAll(req.Body)
        var payload struct{ Preset ComponentPreset `json:"preset"` }
        if err := json.Unmarshal(b, &payload); err != nil {
            t.Fatalf("unmarshal: %v", err)
        }
        if payload.Preset.Name != "Default" || payload.Preset.ComponentID != 7 {
            t.Fatalf("unexpected payload: %+v", payload.Preset)
        }
        res := `{"preset":{"id":11,"name":"Default","component_id":7}}`
        return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(res)), Header: make(http.Header)}, nil
    })}
    created, err := c.CreatePreset(context.Background(), 1, ComponentPreset{Name: "Default", ComponentID: 7})
    if err != nil {
        t.Fatalf("CreatePreset error: %v", err)
    }
    if created.ID != 11 {
        t.Fatalf("unexpected created preset: %+v", created)
    }
    // Update
    c.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
        if req.Method != http.MethodPut {
            t.Fatalf("want PUT, got %s", req.Method)
        }
        if !strings.HasSuffix(req.URL.Path, "/v1/spaces/1/presets/11") {
            t.Fatalf("unexpected path: %s", req.URL.Path)
        }
        res := `{"preset":{"id":11,"name":"Default","component_id":7}}`
        return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(res)), Header: make(http.Header)}, nil
    })}
    upd, err := c.UpdatePreset(context.Background(), 1, ComponentPreset{ID: 11, Name: "Default", ComponentID: 7})
    if err != nil {
        t.Fatalf("UpdatePreset error: %v", err)
    }
    if upd.ID != 11 {
        t.Fatalf("unexpected updated preset: %+v", upd)
    }
}
