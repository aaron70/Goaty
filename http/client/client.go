package ghttpclient

import (
	"context"
	"io"
	"net/http"
	"net/url"

	gerrs "github.com/aaron70/goaty/errors"
	gopts "github.com/aaron70/goaty/options"
)

type NewClientOption func(*Client) error

type Client struct {
	BaseUrl    *url.URL
	httpClient *http.Client
}

func NewClient(base string, options ...NewClientOption) (*Client, error) {
	baseUrl, err := url.Parse(base)
	if err != nil {
		return nil, gerrs.NewError(gerrs.ErrInvalidArgument, err, "Invalid base url given for the client")
	}
	client := &Client{
		BaseUrl: baseUrl,
	}

	if err := gopts.Apply(client, options...); err != nil {
		return nil, err
	}

	if client.httpClient == nil {
		client.httpClient = &http.Client{}
	}

	return client, nil
}

func (c *Client) Url(path string, queryParams url.Values) *url.URL {
	u := c.BaseUrl.JoinPath(path)

	q := u.Query()
	for key, values := range queryParams {
		q[key] = append(q[key], values...)
	}
	u.RawQuery = q.Encode()

	return u
}

type DoRequestOption func(*DoRequest) error

type DoRequest struct {
	Body        io.Reader
	QueryParams url.Values
	Headers     http.Header
}

func (c *Client) Do[B any, E any](ctx context.Context, method, path string, options ...DoRequestOption) (*HttpResponse[B, E], error) {
	doReq := &DoRequest{}
	if err := gopts.Apply(doReq, options...); err != nil {
		return nil, err
	}

	u := c.Url(path, doReq.QueryParams).String()

	req, err := http.NewRequestWithContext(ctx, method, u, doReq.Body)
	if err != nil {
		return nil, gerrs.NewError(gerrs.ErrInvalidArgument, err, "couldn't create %s request to %q", method, u)
	}

	req.Header.Set("Accept", "application/json")
	if doReq.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for key, values := range doReq.Headers {
		req.Header.Del(key)
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, gerrs.NewError(nil, err, "%s %s failed", method, u)
	}

	return NewJsonHttpResponse[B, E](res.StatusCode, res.Body)
}
