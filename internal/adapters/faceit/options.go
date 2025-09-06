package faceit

import "net/http"

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}
