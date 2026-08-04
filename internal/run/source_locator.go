package run

import (
	"strings"
	"unicode/utf8"
)

func validSourceLocator(value string) bool {
	return value != "" && len(value) <= 256 && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}
