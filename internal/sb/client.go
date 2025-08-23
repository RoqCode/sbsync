package sb

import (
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
