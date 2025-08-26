package sb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const base = "https://mapi.storyblok.com/v1"

type Client struct {
	http  *http.Client
	token string
}

func New(token string) *Client {
	return &Client{
		http:  &http.Client{Timeout: 10 * time.Second},
		token: token,
	}
}

// ---------- Spaces ----------
type Space struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	PlanLevel int    `json:"plan_level"`
}

type spacesResp struct {
	Spaces []Space `json:"spaces"`
}

func (c *Client) ListSpaces(ctx context.Context) ([]Space, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/spaces", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	q := req.URL.Query()
	req.URL.RawQuery = q.Encode()

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("spaces.list status " + res.Status)
	}
	var payload spacesResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Spaces, nil
}

// ---------- Stories (flach) ----------

type TranslatedSlug struct {
	ID   *int   `json:"id,omitempty"`
	Lang string `json:"lang"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type Story struct {
	ID                        int              `json:"id,omitempty"`
	UUID                      string           `json:"uuid,omitempty"`
	Name                      string           `json:"name"`
	Slug                      string           `json:"slug"`
	FullSlug                  string           `json:"full_slug"`
	Content                   json.RawMessage  `json:"content,omitempty"`
	ContentType               string           `json:"content_type,omitempty"`
	FolderID                  *int             `json:"parent_id,omitempty"`
	CreatedAt                 string           `json:"created_at,omitempty"`
	UpdatedAt                 string           `json:"updated_at,omitempty"`
	Published                 bool             `json:"published"`
	IsFolder                  bool             `json:"is_folder"`
	IsStartpage               bool             `json:"is_startpage"`
	Position                  int              `json:"position"`
	TagList                   []string         `json:"tag_list,omitempty"`
	TranslatedSlugs           []TranslatedSlug `json:"translated_slugs,omitempty"`
	TranslatedSlugsAttributes []TranslatedSlug `json:"translated_slugs_attributes,omitempty"`
}

type storiesResp struct {
	Stories []Story `json:"stories"`
	Total   int     `json:"total"`
	Page    int     `json:"page"`
	PerPage int     `json:"per_page"`
}

type ListStoriesOpts struct {
	SpaceID int
	Page    int
	PerPage int // 0 => Default 50
	// Optional später: by content type, folder, etc.
}

func (c *Client) ListStories(ctx context.Context, opt ListStoriesOpts) ([]Story, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	if opt.PerPage <= 0 {
		opt.PerPage = 50
	}
	page := opt.Page
	if page <= 0 {
		page = 1
	}

	var all []Story
	for {
		u, _ := url.Parse(base + "/spaces/" + fmt.Sprint(opt.SpaceID) + "/stories")
		q := u.Query()
		q.Set("page", fmt.Sprint(page))
		q.Set("per_page", fmt.Sprint(opt.PerPage))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
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
			return nil, fmt.Errorf("stories.list status %s", res.Status)
		}

		var payload storiesResp
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			return nil, err
		}

		all = append(all, payload.Stories...)

		// Einfacher Abbruch: wenn weniger als PerPage kam, sind wir durch
		if len(payload.Stories) < opt.PerPage {
			break
		}
		page++
	}
	return all, nil
}

// storyResp is used for create/update/get responses.
type storyResp struct {
	Story Story `json:"story"`
}

// storyRawResp mirrors API response when fetching a single story
type storyRawResp struct {
	Story map[string]interface{} `json:"story"`
}

// GetStory fetches a single story by ID.
func (c *Client) GetStory(ctx context.Context, spaceID, storyID int) (Story, error) {
	// Use the same fallback logic as GetStoryWithContent
	return c.GetStoryWithContent(ctx, spaceID, storyID)
}

// CreateStory creates a new story (or folder) in the target space.
func (c *Client) CreateStory(ctx context.Context, spaceID int, st Story) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories", spaceID)
	payload := storyResp{Story: st}
	body, err := json.Marshal(payload)
	if err != nil {
		return Story{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return Story{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return Story{}, fmt.Errorf("story.create status %s", res.Status)
	}
	var resp storyResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Story{}, err
	}
	return resp.Story, nil
}

// UpdateStory updates an existing story in the target space.
func (c *Client) UpdateStory(ctx context.Context, spaceID int, st Story, publish bool) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}
	if st.ID == 0 {
		return Story{}, errors.New("story ID fehlt")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories/%d", spaceID, st.ID)
	payload := map[string]interface{}{
		"story":        st,
		"force_update": "1",
	}
	if !st.IsFolder && publish {
		payload["publish"] = 1
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Story{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewReader(body))
	if err != nil {
		return Story{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return Story{}, fmt.Errorf("failed to read response body: %w", err)
		}
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Error != "" {
			return Story{}, errors.New(apiErr.Error)
		}
		return Story{}, fmt.Errorf("story.update status %s: %s", res.Status, strings.TrimSpace(string(bodyBytes)))
	}
	var resp storyResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Story{}, err
	}
	return resp.Story, nil
}

// CreateStoryWithPublish creates a new story with proper payload structure
func (c *Client) CreateStoryWithPublish(ctx context.Context, spaceID int, st Story, publish bool) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories", spaceID)
	payload := map[string]interface{}{
		"story":        st,
		"force_update": "1",
	}
	if !st.IsFolder && publish {
		payload["publish"] = 1
	}

	// DEBUG: Log the payload before marshalling
	log.Printf("DEBUG: Creating story - Before marshal:")
	log.Printf("DEBUG: Story has content: %t", len(st.Content) > 0)
	if len(st.Content) > 0 {
		log.Printf("DEBUG: Content keys: %v", contentKeysFromRaw(st.Content))
	}
	log.Printf("DEBUG: Story is folder: %t", st.IsFolder)
	log.Printf("DEBUG: Story published: %t", st.Published)

	body, err := json.Marshal(payload)
	if err != nil {
		return Story{}, err
	}

	// DEBUG: Log the actual JSON being sent
	log.Printf("DEBUG: JSON payload being sent (%d bytes):", len(body))
	if len(body) < 2000 {
		log.Printf("DEBUG: Full JSON: %s", string(body))
	} else {
		log.Printf("DEBUG: JSON too large, truncated: %s...", string(body[:2000]))
	}

	// DEBUG: Try minimal payload approach for debugging
	log.Printf("DEBUG: Story UUID: %s", st.UUID)
	log.Printf("DEBUG: Story Name: %s", st.Name)
	log.Printf("DEBUG: Story Slug: %s", st.Slug)
	log.Printf("DEBUG: Story FullSlug: %s", st.FullSlug)
	log.Printf("DEBUG: Story FolderID: %v", st.FolderID)
	log.Printf("DEBUG: Story Published: %t", st.Published)
	log.Printf("DEBUG: Story IsFolder: %t", st.IsFolder)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return Story{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return Story{}, fmt.Errorf("failed to read response body: %w", err)
		}
		// DEBUG: log body for troubleshooting
		if res.StatusCode == 422 {
			log.Printf("DEBUG: 422 Error response body: %s", string(bodyBytes))
		}
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Error != "" {
			return Story{}, errors.New(apiErr.Error)
		}
		return Story{}, fmt.Errorf("story.create status %s: %s", res.Status, strings.TrimSpace(string(bodyBytes)))
	}
	var resp storyResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Story{}, err
	}
	return resp.Story, nil
}

// GetStoriesBySlug finds stories by slug using with_slug parameter
func (c *Client) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]Story, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	u, _ := url.Parse(base + "/spaces/" + fmt.Sprint(spaceID) + "/stories")
	q := u.Query()
	q.Set("with_slug", slug)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
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
		return nil, fmt.Errorf("stories.get_by_slug status %s", res.Status)
	}

	var payload storiesResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Stories, nil
}

// UpdateStoryUUID updates the UUID of a story to maintain identity
func (c *Client) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	if c.token == "" {
		return errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories/%d/update_uuid", spaceID, storyID)
	payload := map[string]string{"uuid": uuid}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return fmt.Errorf("story.update_uuid status %s", res.Status)
	}
	return nil
}

// GetStoryWithContent fetches a story with complete content data
func (c *Client) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}

	// Based on Storyblok CLI: just fetch by ID without version parameter
	// The CLI does: client.get(`spaces/${spaceId}/stories/${storyId}`)
	u := fmt.Sprintf(base+"/spaces/%d/stories/%d", spaceID, storyID)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return Story{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Story{}, fmt.Errorf("story.get status %s", res.Status)
	}
	var payload storyResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Story{}, err
	}

	// DEBUG: Log what we received from the API
	story := payload.Story
	log.Printf("DEBUG: Fetched story %d (%s) - Content present: %t", story.ID, story.FullSlug, len(story.Content) > 0)
	if len(story.Content) > 0 {
		log.Printf("DEBUG: Fetched content keys: %v", contentKeysFromRaw(story.Content))
	} else {
		log.Printf("DEBUG: Story content is empty - this is likely the issue!")
	}

	return story, nil
}

// GetStoryRaw fetches a story and returns the raw map payload for preservation
func (c *Client) GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories/%d", spaceID, storyID)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
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
		return nil, fmt.Errorf("story.get status %s", res.Status)
	}
	var payload storyRawResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Story, nil
}

// CreateStoryRawWithPublish creates a story from a raw map payload, preserving unknown fields
func (c *Client) CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories", spaceID)
	payload := map[string]interface{}{
		"story":        story,
		"force_update": "1",
	}
	// Only publish if not a folder and requested
	if publish {
		if isFolder, ok := story["is_folder"].(bool); !ok || !isFolder {
			payload["publish"] = 1
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Story{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return Story{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(res.Body)
		return Story{}, fmt.Errorf("story.create status %s: %s", res.Status, strings.TrimSpace(string(bodyBytes)))
	}
	var resp storyResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Story{}, err
	}
	return resp.Story, nil
}

// UpdateStoryRawWithPublish updates a story using a raw map payload, preserving unknown fields
func (c *Client) UpdateStoryRawWithPublish(ctx context.Context, spaceID int, storyID int, story map[string]interface{}, publish bool) (Story, error) {
    if c.token == "" {
        return Story{}, errors.New("token leer")
    }
    u := fmt.Sprintf(base+"/spaces/%d/stories/%d", spaceID, storyID)
    payload := map[string]interface{}{
        "story":        story,
        "force_update": "1",
    }
    // Only publish if not a folder and requested
    if publish {
        if isFolder, ok := story["is_folder"].(bool); !ok || !isFolder {
            payload["publish"] = 1
        }
    }
    body, err := json.Marshal(payload)
    if err != nil {
        return Story{}, err
    }
    req, err := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewReader(body))
    if err != nil {
        return Story{}, fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Authorization", c.token)
    req.Header.Add("Content-Type", "application/json")
    res, err := c.http.Do(req)
    if err != nil {
        return Story{}, err
    }
    defer res.Body.Close()
    if res.StatusCode != 200 && res.StatusCode != 201 {
        bodyBytes, _ := io.ReadAll(res.Body)
        return Story{}, fmt.Errorf("story.update status %s: %s", res.Status, strings.TrimSpace(string(bodyBytes)))
    }
    var resp storyResp
    if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
        return Story{}, err
    }
    return resp.Story, nil
}

// getStoryWithVersion fetches story with specific version parameter
func (c *Client) getStoryWithVersion(ctx context.Context, spaceID, storyID int, version string) (Story, error) {
	var u string
	if version == "" {
		// Include resolve_relations to get full content
		u = fmt.Sprintf(base+"/spaces/%d/stories/%d?resolve_relations=1", spaceID, storyID)
	} else {
		// Include both version and resolve_relations for full content
		u = fmt.Sprintf(base+"/spaces/%d/stories/%d?version=%s&resolve_relations=1", spaceID, storyID, version)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return Story{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Story{}, fmt.Errorf("story.get status %s", res.Status)
	}
	var payload storyResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Story{}, err
	}
	return payload.Story, nil
}

// contentKeysFromRaw extracts top-level keys from a JSON raw message.
func contentKeysFromRaw(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var tmp map[string]interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil
	}
	keys := make([]string, 0, len(tmp))
	for k := range tmp {
		keys = append(keys, k)
	}
	return keys
}
