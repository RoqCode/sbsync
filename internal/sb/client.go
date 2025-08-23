package sb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type StoryblokClient struct {
	Token      string
	httpClient *http.Client
}

type SpacesResponse struct {
	Spaces []Space `json:"spaces"`
}

type Space struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

func (c *StoryblokClient) ListSpaces() {
	req, err := http.NewRequest("GET", "https://mapi.storyblok.com/v1/spaces/", nil)
	if err != nil {
		fmt.Println("[ListSpaces]: error building request:", err)
		return
	}

	req.Header.Set("Authorization", c.Token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Println("[ListSpaces]: request error:", err)
		return
	}

	defer resp.Body.Close()
	fmt.Println("[ListSpaces] status:", resp.Status)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[ListSpaces]: error reading body:", err)
		return
	}

	var out SpacesResponse
	err = json.Unmarshal(body, &out)
	if err != nil {
		fmt.Println("[ListSpaces]: error parsing body:", err)
		return
	}

	for _, s := range out.Spaces {
		fmt.Printf("- %s (id=%d, region=%s)\n", s.Name, s.ID, s.Region)
	}
}

func NewStoryblokClient(token string) *StoryblokClient {
	return &StoryblokClient{
		Token:      token,
		httpClient: &http.Client{},
	}
}
