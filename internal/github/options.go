package github

import "net/http"

type WithBaseURL string

func (w WithBaseURL) ConfigureClient(c *ClientConfig) {
	c.BaseURL = string(w)
}

type WithClient http.Client

func (w WithClient) ConfigureClient(c *ClientConfig) {
	c.Client = http.Client(w)
}
