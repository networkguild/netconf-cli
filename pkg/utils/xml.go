package utils

import (
	"strings"

	"github.com/go-xmlfmt/xmlfmt"
)

func FormatXML(input string) string {
	input = strings.TrimFunc(input, func(r rune) bool {
		if r == '\n' || r == ' ' || r == '\t' {
			return true
		}
		return false
	})

	return strings.TrimPrefix(xmlfmt.FormatXML(input, "", "  "), "\n")
}
