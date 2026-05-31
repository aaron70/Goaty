package utils

import "log"

func PanicErr(err error) {
	if err != nil {
		panic(err)
	}
}

func Must[T any](v T, err error) T {
	PanicErr(err)
	return v
}


func LogDefaultErr(err error) error {
	if err != nil {
		log.Printf("Error: %s\n", err)
	}
	return err
}
