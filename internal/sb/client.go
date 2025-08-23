// package sb
//
// import (
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// )
//
// type StoryblokClient struct {
// 	Token      string
// 	httpClient *http.Client
// }
//
// type SpacesResponse struct {
// 	Spaces []Space `json:"spaces"`
// }
//
// type Space struct {
// 	ID     int    `json:"id"`
// 	Name   string `json:"name"`
// 	Region string `json:"region"`
// }
//
// func (c *StoryblokClient) ListSpaces() {
// 	req, err := http.NewRequest("GET", "https://mapi.storyblok.com/v1/spaces/", nil)
// 	if err != nil {
// 		fmt.Println("[ListSpaces]: error building request:", err)
// 		return
// 	}
//
// 	req.Header.Set("Authorization", c.Token)
// 	req.Header.Add("Content-Type", "application/json")
//
// 	resp, err := c.httpClient.Do(req)
// 	if err != nil {
// 		fmt.Println("[ListSpaces]: request error:", err)
// 		return
// 	}
//
// 	defer resp.Body.Close()
// 	fmt.Println("[ListSpaces] status:", resp.Status)
//
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		fmt.Println("[ListSpaces]: error reading body:", err)
// 		return
// 	}
//
// 	var out SpacesResponse
// 	err = json.Unmarshal(body, &out)
// 	if err != nil {
// 		fmt.Println("[ListSpaces]: error parsing body:", err)
// 		return
// 	}
//
// 	for _, s := range out.Spaces {
// 		fmt.Printf("- %s (id=%d, region=%s)\n", s.Name, s.ID, s.Region)
// 	}
// }
//
// func NewStoryblokClient(token string) *StoryblokClient {
// 	return &StoryblokClient{
// 		Token:      token,
// 		httpClient: &http.Client{},
// 	}
// }

package sb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
