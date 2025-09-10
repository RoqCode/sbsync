package sb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// Component represents a Storyblok component definition
type Component struct {
	ID                 int               `json:"id,omitempty"`
	Name               string            `json:"name"`
	DisplayName        string            `json:"display_name,omitempty"`
	Schema             json.RawMessage   `json:"schema,omitempty"`
	ComponentGroupUUID string            `json:"component_group_uuid,omitempty"`
	InternalTagIDs     IntSlice          `json:"internal_tag_ids,omitempty"`
	InternalTagsList   []InternalTag     `json:"internal_tags_list,omitempty"`
	AllPresets         []ComponentPreset `json:"all_presets,omitempty"`
	CreatedAt          string            `json:"created_at,omitempty"`
	UpdatedAt          string            `json:"updated_at,omitempty"`
}

// IntSlice unmarshals JSON arrays containing numbers or numeric strings into []int
type IntSlice []int

func (s *IntSlice) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = nil
		return nil
	}
	var ints []int
	if err := json.Unmarshal(b, &ints); err == nil {
		*s = ints
		return nil
	}
	var arr []interface{}
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	out := make([]int, 0, len(arr))
	for _, v := range arr {
		switch vv := v.(type) {
		case float64:
			out = append(out, int(vv))
		case string:
			if vv == "" {
				continue
			}
			if n, err := strconv.Atoi(vv); err == nil {
				out = append(out, n)
			}
		}
	}
	*s = out
	return nil
}

type componentsResp struct {
	Components []Component `json:"components"`
}

type componentResp struct {
	Component Component `json:"component"`
}

// ComponentPreset represents a saved preset for a component
type ComponentPreset struct {
	ID          int             `json:"id,omitempty"`
	Name        string          `json:"name"`
	ComponentID int             `json:"component_id"`
	Preset      json.RawMessage `json:"preset,omitempty"`
	Image       string          `json:"image,omitempty"`
}

type presetsResp struct {
	Presets []ComponentPreset `json:"presets"`
}

type presetResp struct {
	Preset ComponentPreset `json:"preset"`
}

// ListComponents lists components for a space
func (c *Client) ListComponents(ctx context.Context, spaceID int) ([]Component, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/components", spaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("components.list status %s", res.Status)
	}
	var payload componentsResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Components, nil
}

// CreateComponent creates a component
func (c *Client) CreateComponent(ctx context.Context, spaceID int, comp Component) (Component, error) {
	if c.token == "" {
		return Component{}, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/components", spaceID)
	payload := map[string]interface{}{"component": comp}
	body, err := json.Marshal(payload)
	if err != nil {
		return Component{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return Component{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Component{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return Component{}, fmt.Errorf("component.create status %s", res.Status)
	}
	var resp componentResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Component{}, err
	}
	return resp.Component, nil
}

// UpdateComponent updates a component by ID
func (c *Client) UpdateComponent(ctx context.Context, spaceID int, comp Component) (Component, error) {
	if c.token == "" {
		return Component{}, errors.New("token leer")
	}
	if comp.ID == 0 {
		return Component{}, errors.New("component id required")
	}
	u := fmt.Sprintf(base+"/spaces/%d/components/%d", spaceID, comp.ID)
	payload := map[string]interface{}{"component": comp}
	body, err := json.Marshal(payload)
	if err != nil {
		return Component{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return Component{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Component{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return Component{}, fmt.Errorf("component.update status %s", res.Status)
	}
	var resp componentResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Component{}, err
	}
	return resp.Component, nil
}

// ComponentGroup represents a component group
type ComponentGroup struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type componentGroupsResp struct {
	ComponentGroups []ComponentGroup `json:"component_groups"`
}

type componentGroupResp struct {
	ComponentGroup ComponentGroup `json:"component_group"`
}

// ListComponentGroups lists component groups for a space
func (c *Client) ListComponentGroups(ctx context.Context, spaceID int) ([]ComponentGroup, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/component_groups", spaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("component_groups.list status %s", res.Status)
	}
	var payload componentGroupsResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.ComponentGroups, nil
}

// CreateComponentGroup creates a component group by name
func (c *Client) CreateComponentGroup(ctx context.Context, spaceID int, name string) (ComponentGroup, error) {
	if c.token == "" {
		return ComponentGroup{}, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/component_groups", spaceID)
	payload := map[string]interface{}{"component_group": map[string]string{"name": name}}
	body, err := json.Marshal(payload)
	if err != nil {
		return ComponentGroup{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return ComponentGroup{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return ComponentGroup{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return ComponentGroup{}, fmt.Errorf("component_groups.create status %s", res.Status)
	}
	var resp componentGroupResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return ComponentGroup{}, err
	}
	return resp.ComponentGroup, nil
}

// InternalTag represents an internal tag
type InternalTag struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ObjectType string `json:"object_type"`
}

type internalTagsResp struct {
	InternalTags []InternalTag `json:"internal_tags"`
}

type internalTagResp struct {
	InternalTag InternalTag `json:"internal_tag"`
}

// ListInternalTags lists internal tags for a space
func (c *Client) ListInternalTags(ctx context.Context, spaceID int) ([]InternalTag, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/internal_tags", spaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("internal_tags.list status %s", res.Status)
	}
	var payload internalTagsResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.InternalTags, nil
}

// CreateInternalTag creates an internal tag for a given object type
func (c *Client) CreateInternalTag(ctx context.Context, spaceID int, name string, objectType string) (InternalTag, error) {
	if c.token == "" {
		return InternalTag{}, errors.New("token leer")
	}
	if objectType == "" {
		objectType = "component"
	}
	u := fmt.Sprintf(base+"/spaces/%d/internal_tags", spaceID)
	payload := map[string]interface{}{"internal_tag": map[string]string{"name": name, "object_type": objectType}}
	body, err := json.Marshal(payload)
	if err != nil {
		return InternalTag{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return InternalTag{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return InternalTag{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return InternalTag{}, fmt.Errorf("internal_tags.create status %s", res.Status)
	}
	var resp internalTagResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return InternalTag{}, err
	}
	return resp.InternalTag, nil
}

// ListPresets lists all presets for a space
func (c *Client) ListPresets(ctx context.Context, spaceID int) ([]ComponentPreset, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/presets", spaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("presets.list status %s", res.Status)
	}
	var payload presetsResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Presets, nil
}

// CreatePreset creates a preset in a space for a component
func (c *Client) CreatePreset(ctx context.Context, spaceID int, p ComponentPreset) (ComponentPreset, error) {
	if c.token == "" {
		return ComponentPreset{}, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/presets", spaceID)
	payload := map[string]interface{}{"preset": p}
	body, err := json.Marshal(payload)
	if err != nil {
		return ComponentPreset{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return ComponentPreset{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return ComponentPreset{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return ComponentPreset{}, fmt.Errorf("preset.create status %s", res.Status)
	}
	var resp presetResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return ComponentPreset{}, err
	}
	return resp.Preset, nil
}

// UpdatePreset updates an existing preset by ID
func (c *Client) UpdatePreset(ctx context.Context, spaceID int, p ComponentPreset) (ComponentPreset, error) {
	if c.token == "" {
		return ComponentPreset{}, errors.New("token leer")
	}
	if p.ID == 0 {
		return ComponentPreset{}, errors.New("preset id required")
	}
	u := fmt.Sprintf(base+"/spaces/%d/presets/%d", spaceID, p.ID)
	payload := map[string]interface{}{"preset": p}
	body, err := json.Marshal(payload)
	if err != nil {
		return ComponentPreset{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return ComponentPreset{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return ComponentPreset{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return ComponentPreset{}, fmt.Errorf("preset.update status %s", res.Status)
	}
	var resp presetResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return ComponentPreset{}, err
	}
	return resp.Preset, nil
}
