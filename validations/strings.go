package validations

import "strings"

func StrIsBlank(str string) bool {
	return strings.Trim(str, " ") == ""
} 
