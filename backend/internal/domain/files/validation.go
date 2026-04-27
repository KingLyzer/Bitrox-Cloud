package files

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxNodeNameLength = 255

func ValidateNodeName(name string) error {
	normalized := NormalizeNodeName(name)
	if normalized == "" {
		return ErrInvalidName
	}
	if utf8.RuneCountInString(normalized) > maxNodeNameLength {
		return fmt.Errorf("%w: name too long", ErrInvalidName)
	}
	if normalized == "." || normalized == ".." {
		return fmt.Errorf("%w: reserved name", ErrInvalidName)
	}
	if strings.ContainsAny(normalized, "/\\\x00") {
		return fmt.Errorf("%w: illegal character", ErrInvalidName)
	}
	return nil
}
