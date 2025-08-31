package sb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

const cdaBase = "https://api.storyblok.com/v2/cdn"

// CDAClient is a minimal Content Delivery API client using the shared transport.
type CDAClient struct {
	http  *http.Client
	token string // public/preview token
}

// NewCDA creates a CDA client reusing the retrying/limited transport.
func NewCDA(token string) *CDAClient {
	opts := DefaultTransportOptionsFromEnv()
	rt := NewRetryingLimiterTransport(opts)
	return &CDAClient{
		http:  &http.Client{Transport: rt, Timeout: 0},
		token: token,
	}
}

// NewCDAWithOptions creates a CDA client with custom transport options (tests).
func NewCDAWithOptions(token string, opts TransportOptions) *CDAClient {
	rt := NewRetryingLimiterTransport(opts)
	return &CDAClient{http: &http.Client{Transport: rt, Timeout: 0}, token: token}
}

// cdaStoryResp mirrors CDA single story response.
type cdaStoryResp struct {
	Story map[string]any `json:"story"`
}

type cdaStoriesResp struct {
	Stories []map[string]any `json:"stories"`
	Total   int              `json:"total"`
	PerPage int              `json:"per_page"`
	Page    int              `json:"page"`
}

// GetStoryRawBySlug fetches a story by slug from CDA (version optional: "published" or "draft").
func (c *CDAClient) GetStoryRawBySlug(ctx context.Context, spaceID int, slug string, version string) (map[string]any, error) {
	return c.GetStoryRawBySlugLang(ctx, spaceID, slug, version, "")
}

// GetStoryRawBySlugLang fetches a story by slug and language from CDA (language optional).
func (c *CDAClient) GetStoryRawBySlugLang(ctx context.Context, spaceID int, slug string, version string, language string) (map[string]any, error) {
	if c.token == "" {
		return nil, errors.New("cda token empty")
	}
	if version == "" {
		version = "published"
	}
	u, _ := url.Parse(cdaBase + "/stories/" + slug)
	q := u.Query()
	q.Set("token", c.token)
	q.Set("version", version)
	q.Set("cv", "0") // placeholder; snapshotting may pass real cv later
	if language != "" {
		q.Set("language", language)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cda.story.get status %s", res.Status)
	}
	var payload cdaStoryResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Story, nil
}

// WalkStoriesByPrefix iterates all stories under the given starts_with prefix,
// invoking fn for each story across all pages (per_page up to 100).
func (c *CDAClient) WalkStoriesByPrefix(ctx context.Context, startsWith, version string, perPage int, fn func(map[string]any) error) error {
	return c.WalkStoriesByPrefixLang(ctx, startsWith, version, "", perPage, fn)
}

// WalkStoriesByPrefixLang iterates all stories for a given language
func (c *CDAClient) WalkStoriesByPrefixLang(ctx context.Context, startsWith, version, language string, perPage int, fn func(map[string]any) error) error {
	if c.token == "" {
		return errors.New("cda token empty")
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 100
	}
	page := 1
	for {
		u, _ := url.Parse(cdaBase + "/stories")
		q := u.Query()
		q.Set("token", c.token)
		if version == "" {
			version = "published"
		}
		q.Set("version", version)
		q.Set("starts_with", startsWith)
		q.Set("per_page", fmt.Sprint(perPage))
		q.Set("page", fmt.Sprint(page))
		if language != "" {
			q.Set("language", language)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		if res.Body != nil {
			defer res.Body.Close()
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("cda.stories.list status %s", res.Status)
		}
		var payload cdaStoriesResp
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			return err
		}
		for _, st := range payload.Stories {
			if err := fn(st); err != nil {
				return err
			}
		}
		if len(payload.Stories) < perPage {
			break
		}
		page++
	}
	return nil
}
