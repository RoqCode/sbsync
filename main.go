package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var TOKEN_PATH = "./.sbrc"

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

func main() {
	rcFile, err := os.Open(TOKEN_PATH)
	if err != nil {
		fmt.Println("[file reader]: error finding rc:", err)
		return
	}
	defer rcFile.Close()

	rcContent, err := io.ReadAll(rcFile)
	if err != nil {
		fmt.Println("[file reader]: error reading rc:", err)
		return
	}

	parts := strings.SplitN(string(rcContent), "=", 2)
	if len(parts) != 2 {
		fmt.Println("[file reader]: invalid format in rc file")
		return
	}
	token := strings.TrimSpace(parts[1])

	client := NewStoryblokClient(token)
	client.ListSpaces()
}
