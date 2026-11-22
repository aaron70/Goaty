package ghttpclient

import (
	"encoding/json"
	"io"

	gerrs "github.com/aaron70/goaty/errors"
	gopts "github.com/aaron70/goaty/options"
)

type HttpResponseOption[B, E any] func(*HttpResponse[B, E]) error

type HttpResponse[B, E any] struct {
	StatusCode int
	body       io.ReadCloser

	BodyParser func(io.Reader) (B, error)
	ErrParser func(io.Reader) (E, error)
}

func NewHttpResponse[B, E any](statusCode int, body io.ReadCloser, options ...HttpResponseOption[B, E]) (*HttpResponse[B, E], error) {
	res := &HttpResponse[B, E]{
		StatusCode: statusCode,
		body: body,
	}
	if err := gopts.Apply(res, options...); err != nil {
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

func (r *HttpResponse[B, E]) Body() (B, error) {
	var zero B
	if r.BodyParser == nil {
		return zero, gerrs.NewError(gerrs.ErrNilReference, nil, "Can't serialize the body, the BodyParser is nil")
	}

	defer r.body.Close()
	parsed, err := r.BodyParser(r.body)
	if err != nil {
		return zero, gerrs.NewError(gerrs.ErrSerilization, err, "The body couldn't be deserialzied")
	}

	return parsed, nil
}

func (r *HttpResponse[B, E]) Error() (E, error) {
	var zero E
	if r.ErrParser == nil {
		return zero, gerrs.NewError(gerrs.ErrNilReference, nil, "Can't serialize the body, the ErrParser is nil")
	}

	defer r.body.Close()
	parsed, err := r.ErrParser(r.body)
	if err != nil {
		return zero, gerrs.NewError(gerrs.ErrSerilization, err, "The body couldn't be deserialzied")
	}

	return parsed, nil
}

func JsonParser[T any](r io.Reader) (res T, err error) {
	err = json.NewDecoder(r).Decode(&res)
	return res, err
}
