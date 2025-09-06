package faceit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBase = "https://open.faceit.com/data/v4"

type Client struct {
	apiKey  string
	http    *http.Client
	baseURL string
}

func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 10 * time.Second},
		baseURL: defaultBase,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// doJSON: construye URL, agrega Authorization, maneja 404 y 429 con Retry-After simple.
func (c *Client) doJSON(ctx context.Context, method, path string, q url.Values, out any) error {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, method, u, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("faceit http: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		// backoff básico leyendo Retry-After (segundos)
		if ra := res.Header.Get("Retry-After"); ra != "" {
			if sec, _ := strconv.Atoi(ra); sec > 0 {
				select {
				case <-time.After(time.Duration(sec) * time.Second):
				case <-ctx.Done():
					return ctx.Err()
				}
				// un reintento
				return c.doJSON(ctx, method, path, q, out)
			}
		}
	}

	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return &APIError{Status: res.StatusCode, Body: strings.TrimSpace(string(b))}
	}

	return json.NewDecoder(res.Body).Decode(out)
}
