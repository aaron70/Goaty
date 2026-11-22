package gslices

func Map[O, T any](s []O, f func(O) (T, error)) ([]T, error) {
	target := make([]T, len(s))
	for i := range s {
		mapped, err := f(s[i])
		if err != nil {
			return nil, err
		}
		target[i] = mapped
	}
	return target, nil
}
