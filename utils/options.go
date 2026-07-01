package utils

type Option[C any] = func(*C) error

func ApplyOptions[C any, O ~Option[C]](config *C, options ...O) error {
	var zero C
	if config == nil {
		config = &zero
	}

	for _, option := range options {
		if err := option(config); err != nil {
			return err
		}
	}

	return nil
}
