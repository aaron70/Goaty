package ghttpclient

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"

	gopts "github.com/aaron70/goaty/options"
)

type Client struct {
	BaseUrl string
	httpClient *http.Client
}

func (c *Client) Url(path string, queryParams url.Values) (string, error) {
	fullUrl, err := url.JoinPath(c.BaseUrl, url.PathEscape(path))	
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s?%s", fullUrl, queryParams.Encode()), nil
}

type DoRequestOption func(*DoRequest) error

type DoRequest struct {
	Body io.Reader
	QueryParams url.Values
	Headers http.Header
}


func (c *Client) Do[B any, E any](ctx context.Context, method, path string, options ...DoRequestOption) (*HttpResponse[B, E], error) {
	doReq := &DoRequest{} 
	if err := gopts.Apply(doReq, options...); err != nil {
		return nil, err
	}

	urlPath, err := c.Url(path, doReq.QueryParams)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, urlPath, doReq.Body)
	if err != nil {
		return nil, err
	}
	maps.Insert(req.Header, maps.All(doReq.Headers))

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return NewJsonHttpResponse[B, E](res.StatusCode, res.Body)
}

