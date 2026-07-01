package options

import "github.com/aaron70/goaty/errors"

type Option[C any] interface {
	Apply(*C) error
}

type OptionAny interface {
	Apply(any) error
}

type OptionFunc[C any] func(*C) error
func (o OptionFunc[C]) Apply(config *C) error {
	return o(config)
}

type OptionAnyFunc func(any) error
func (o OptionAnyFunc) Apply(config any) error {
	return o(config)
}


func ApplyOptions[C any](options []Option[C], config C) (C, error) {
	for _, option := range options {
		if err := option.Apply(&config); err != nil {
			return config, err
		}
	}
	return config, nil
}

func ApplyAnyOptions[C any](options []OptionAny, config C) (C, error) {
	for _, option := range options {
		if err := option.Apply(config); err != nil {
			return config, err
		}
	}
	return config, nil
}

func CastOptionAny[C any](opt OptionAny, v any) error {
	var ref C
	config, ok := v.(*C)
	if !ok {
		return opt.Apply(config)
	}
	return errors.NewDevError(errors.ErrInvalidArgument, nil, "Invalid options configuration, expected %T got: %T", ref, v)
}
