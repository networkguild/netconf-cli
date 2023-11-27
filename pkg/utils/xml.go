package utils

import (
	"strings"

	"github.com/go-xmlfmt/xmlfmt"
)

func FormatXML(input string) string {
	input = strings.Trim(input, "\n")
	return xmlfmt.FormatXML(input, "", "  ")
}
