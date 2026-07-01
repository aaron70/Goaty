package repositories

import (
	"os"

	"github.com/aaron70/goaty/errors"
)

var (
	FSSaveOptions = fSSaveOptions{}
)

type fSSaveOptionConfig struct {
	Perms os.FileMode
}

type fSSaveOption func(*fSSaveOptionConfig) error

func (o fSSaveOption) saveOption(v any) error {
	config, ok := v.(*fSSaveOptionConfig)
	if ok {
		return o(config)
	} else if !IgnoreUnknownOptions {
		return errors.NewDevError(errors.ErrInvalidArgument, nil, "Invalid configuration for FileSystem SaveOption expected *fSSaveOptionConfig got: %T", v)
	} else {
		return nil
	}
}

type fSSaveOptions struct{}

func (o fSSaveOptions) WithPermissions(perm os.FileMode) fSSaveOption {
	return func(fsoc *fSSaveOptionConfig) error {
		fsoc.Perms = perm
		return nil
	}
}
