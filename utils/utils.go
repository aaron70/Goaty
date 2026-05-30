package utils

func PanicErr(err error) {
	if err != nil {
		panic(err)
	}
}

func Must[T any](v T, err error) T {
	PanicErr(err)
	return v
}

