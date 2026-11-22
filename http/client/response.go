package ghttpclient

import (
	"encoding/json"
	"errors"
	"io"

	gerrs "github.com/aaron70/goaty/errors"
	gopts "github.com/aaron70/goaty/options"
)

const maxDrainBytes = 64 << 10

type HttpResponseOption[B, E any] func(*HttpResponse[B, E]) error

type HttpResponse[B, E any] struct {
	StatusCode int
	body       io.ReadCloser

	BodyParser func(io.Reader) (B, error)
	ErrParser  func(io.Reader) (E, error)
}

func NewHttpResponse[B, E any](statusCode int, body io.ReadCloser, options ...HttpResponseOption[B, E]) (*HttpResponse[B, E], error) {
	res := &HttpResponse[B, E]{
		StatusCode: statusCode,
		body:       body,
	}
	if err := gopts.Apply(res, options...); err != nil {
		_ = res.Close()
		return nil, err
	}
	return res, nil
}

func NewJsonHttpResponse[B, E any](statusCode int, body io.ReadCloser, options ...HttpResponseOption[B, E]) (*HttpResponse[B, E], error) {
	options = append(options, HttpResponseWithBodyParser[B, E](JsonParser), HttpResponseWithErrParser[B, E](JsonParser))
	return NewHttpResponse[B, E](statusCode, body, options...)
}

func HttpResponseWithBodyParser[B, E any](parser func(io.Reader) (B, error)) HttpResponseOption[B, E] {
	return func(r *HttpResponse[B, E]) error {
		r.BodyParser = parser
		return nil
	}
}

func HttpResponseWithErrParser[B, E any](parser func(io.Reader) (E, error)) HttpResponseOption[B, E] {
	return func(r *HttpResponse[B, E]) error {
		r.ErrParser = parser
		return nil
	}
}

func (r *HttpResponse[B, E]) IsError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 600
}

func (r *HttpResponse[B, E]) RawBody() io.ReadCloser {
	return r.body
}

// Close releases the underlying body. It drains a bounded amount of unread
// data first so the connection can be reused. It is safe to call more than once.
// Call it when the body is not consumed through Body or Error.
func (r *HttpResponse[B, E]) Close() error {
	if r.body == nil {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(r.body, maxDrainBytes))
	return r.body.Close()
}

func (r *HttpResponse[B, E]) Body() (B, error) {
	var zero B
	defer r.Close()

	if r.body == nil {
		return zero, gerrs.NewError(gerrs.ErrNilReference, nil, "Can't serialize the body, the body is nil")
	}
	if r.BodyParser == nil {
		return zero, gerrs.NewError(gerrs.ErrNilReference, nil, "Can't serialize the body, the BodyParser is nil")
	}

	parsed, err := r.BodyParser(r.body)
	if err != nil {
		return zero, gerrs.NewError(gerrs.ErrSerilization, err, "The body couldn't be deserialzied")
	}

	return parsed, nil
}

func (r *HttpResponse[B, E]) Error() (E, error) {
	var zero E
	defer r.Close()

	if r.body == nil {
		return zero, gerrs.NewError(gerrs.ErrNilReference, nil, "Can't serialize the error body, the body is nil")
	}
	if r.ErrParser == nil {
		return zero, gerrs.NewError(gerrs.ErrNilReference, nil, "Can't serialize the error body, the ErrParser is nil")
	}

	parsed, err := r.ErrParser(r.body)
	if err != nil {
		return zero, gerrs.NewError(gerrs.ErrSerilization, err, "The body couldn't be deserialzied")
	}

	return parsed, nil
}

func JsonParser[T any](r io.Reader) (res T, err error) {
	err = json.NewDecoder(r).Decode(&res)
	if errors.Is(err, io.EOF) {
		return res, nil
	}
	return res, err
}
