package repositories

import (
	"os"

	"github.com/aaron70/goaty/options"
)

var (
	FSSaveOptions = fsSaveOptions{}
)

type fsSaveConfig struct {
	Perm os.FileMode
}

type fsSaveOption func(*fsSaveConfig) error
func (o fsSaveOption) Apply(v any) error {
	return options.CastOptionAny[fsSaveConfig](o, v)
}

var _ options.OptionAny = fsSaveOption(func(fsc *fsSaveConfig) error { return nil })

type fsSaveOptions struct {}

func (o fsSaveOptions) WithFileMode(perm os.FileMode) fsSaveOption {
	return func(fsc *fsSaveConfig) error {
		fsc.Perm = perm
		return nil
	}
}
