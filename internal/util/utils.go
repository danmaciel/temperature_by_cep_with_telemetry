package util

import (
	"net/url"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func StringPrepare(s string) string {

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	text, _, _ := transform.String(t, s)

	return url.QueryEscape(text)
}
