package sb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type spacesResp struct {
	Spaces []Space `json:"spaces"`
}

func (c *Client) ListSpaces(ctx context.Context) ([]Space, error) {
	if c.token == "" {
		return nil, errors.New("token leer")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/spaces", nil)
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

type Story struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	FullSlug    string `json:"full_slug"`
	FolderID    *int   `json:"parent_id"`
	UpdatedAt   string `json:"updated_at"`
	IsFolder    bool   `json:"is_folder"`
	IsStartpage bool   `json:"is_startpage"`
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

		req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		req.Header.Set("Authorization", c.token)
		req.Header.Add("Content-Type", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}

		if res.StatusCode != 200 {
			res.Body.Close()
			return nil, fmt.Errorf("stories.list status %s", res.Status)
		}

		var payload storiesResp
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			res.Body.Close()
			return nil, err
		}
		res.Body.Close()

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

// GetStory fetches a single story by ID.
func (c *Client) GetStory(ctx context.Context, spaceID, storyID int) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories/%d", spaceID, storyID)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
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

// GetStoryBySlug fetches a single story by its full slug.
func (c *Client) GetStoryBySlug(ctx context.Context, spaceID int, slug string) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}
	u, _ := url.Parse(fmt.Sprintf(base+"/spaces/%d/stories", spaceID))
	q := u.Query()
	q.Set("with_slug", slug)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return Story{}, nil
	}
	if res.StatusCode != 200 {
		return Story{}, fmt.Errorf("story.slug status %s", res.Status)
	}
	var payload storyResp
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Story{}, err
	}
	return payload.Story, nil
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
	req, _ := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
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
func (c *Client) UpdateStory(ctx context.Context, spaceID int, st Story) (Story, error) {
	if c.token == "" {
		return Story{}, errors.New("token leer")
	}
	if st.ID == 0 {
		return Story{}, errors.New("story ID fehlt")
	}
	u := fmt.Sprintf(base+"/spaces/%d/stories/%d", spaceID, st.ID)
	payload := storyResp{Story: st}
	body, err := json.Marshal(payload)
	if err != nil {
		return Story{}, err
	}
	req, _ := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewReader(body))
	req.Header.Set("Authorization", c.token)
	req.Header.Add("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Story{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return Story{}, fmt.Errorf("story.update status %s", res.Status)
	}
	var resp storyResp
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Story{}, err
	}
	return resp.Story, nil
}
