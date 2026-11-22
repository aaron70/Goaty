package ghttpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Minimal helpers so the tests run without downloading testify.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func eq(t *testing.T, want, got any, msg string) {
	t.Helper()
	if want != got {
		t.Fatalf("%s: want %#v, got %#v", msg, want, got)
	}
}

func isTrue(t *testing.T, ok bool, msg string) {
	t.Helper()
	if !ok {
		t.Fatal(msg)
	}
}

type user struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type apiErr struct {
	Message string `json:"message"`
}

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL + "/v1")
	must(t, err)
	return c
}

func TestUrl_JoinsPathWithoutEscaping(t *testing.T) {
	c, err := NewClient("https://api.example.com/v1")
	must(t, err)

	got := c.Url("users/1", nil)
	eq(t, "https://api.example.com/v1/users/1", got.String(), "value")
}

func TestUrl_NoTrailingQuestionMark(t *testing.T) {
	c, err := NewClient("https://api.example.com")
	must(t, err)

	got := c.Url("users", url.Values{})
	eq(t, "https://api.example.com/users", got.String(), "value")
}

func TestUrl_MergesBaseQuery(t *testing.T) {
	c, err := NewClient("https://api.example.com/v1?key=abc")
	must(t, err)

	u := c.Url("users", url.Values{"page": {"2"}})

	eq(t, "/v1/users", u.Path, "value")
	eq(t, "abc", u.Query().Get("key"), "value")
	eq(t, "2", u.Query().Get("page"), "value")
}

func TestDo_SendsDefaultsAndCallerHeaders(t *testing.T) {
	var got http.Header
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"id":1,"name":"a"}`)
	})

	_, err := c.Do[user, apiErr](context.Background(), http.MethodPost, "users",
		func(d *DoRequest) error {
			d.Body = strings.NewReader(`{}`)
			d.Headers = http.Header{"accept": {"text/plain"}, "x-custom": {"1"}}
			return nil
		},
	)
	must(t, err)

	eq(t, "text/plain", got.Get("Accept"), "caller header should override default")
	eq(t, "1", got.Get("X-Custom"), "non-canonical caller key should be canonicalized")
	eq(t, "application/json", got.Get("Content-Type"), "default Content-Type when body is set")
}

func TestDo_DefaultAcceptWithoutBody(t *testing.T) {
	var got http.Header
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	})

	res, err := c.Do[user, apiErr](context.Background(), http.MethodGet, "users")
	must(t, err)
	defer res.Close()

	eq(t, "application/json", got.Get("Accept"), "value")
	eq(t, "", got.Get("Content-Type"), "should be empty")
}

func TestDo_NoContentBodyParsesToZero(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	res, err := c.Do[user, apiErr](context.Background(), http.MethodDelete, "users/1")
	must(t, err)

	b, err := res.Body()
	must(t, err)
	eq(t, user{}, b, "value")
}

func TestDo_ErrorResponse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"not found"}`)
	})

	res, err := c.Do[user, apiErr](context.Background(), http.MethodGet, "users/1")
	must(t, err)
	isTrue(t, res.IsError(), "expected true")

	e, err := res.Error()
	must(t, err)
	eq(t, "not found", e.Message, "value")
}

func TestDo_CloseWithoutConsuming(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":1}`)
	})

	res, err := c.Do[user, apiErr](context.Background(), http.MethodGet, "users/1")
	must(t, err)
	must(t, res.Close())
	eq(t, nil, res.Close(), "Close must be idempotent")
}

func TestHttpResponse_NilBody(t *testing.T) {
	res, err := NewJsonHttpResponse[user, apiErr](http.StatusOK, nil)
	must(t, err)

	_, err = res.Body()
	isTrue(t, err != nil, "expected error")

	_, err = res.Error()
	isTrue(t, err != nil, "expected error")
	must(t, res.Close())
}

func TestDo_InvalidBaseUrl(t *testing.T) {
	c := &Client{BaseUrl: &url.URL{Scheme: "://bad"}, httpClient: &http.Client{}}

	_, err := c.Do[user, apiErr](context.Background(), http.MethodGet, "users")
	isTrue(t, err != nil, "expected error")
}
