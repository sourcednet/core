// Package htmllink finds a page's sourced-record link, in its HTML head or in
// its HTTP Link header.
package htmllink

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Rel is the link relation pointing a page to its current record.
const Rel = "sourced-record"

// FromHTML returns the absolute href of the page's sourced-record link, or
// "" if it has none. It reads only as far as the end of <head>.
func FromHTML(body []byte, pageURL string) string {
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return ""
		case html.EndTagToken:
			if name, _ := z.TagName(); atom.Lookup(name) == atom.Head {
				return ""
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Body:
				return ""
			case atom.Link:
			default:
				continue
			}
			if !hasAttr {
				continue
			}
			var rel, href string
			for more := true; more; {
				var k, v []byte
				k, v, more = z.TagAttr()
				switch string(k) {
				case "rel":
					rel = string(v)
				case "href":
					href = string(v)
				}
			}
			if hasRel(rel) {
				return resolve(pageURL, href)
			}
		}
	}
}

// FromHeader returns the absolute target of a sourced-record Link header
// (RFC 8288), or "" if there is none.
func FromHeader(h http.Header, pageURL string) string {
	for _, v := range h.Values("Link") {
		for _, link := range strings.Split(v, ",") {
			target, params, ok := strings.Cut(strings.TrimSpace(link), ";")
			target = strings.TrimSpace(target)
			if !ok || !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
				continue
			}
			for _, p := range strings.Split(params, ";") {
				k, val, _ := strings.Cut(strings.TrimSpace(p), "=")
				if strings.EqualFold(k, "rel") && hasRel(strings.Trim(val, `"`)) {
					return resolve(pageURL, target[1:len(target)-1])
				}
			}
		}
	}
	return ""
}

// hasRel reports whether a space-separated rel value includes Rel.
func hasRel(rel string) bool {
	for _, r := range strings.Fields(rel) {
		if strings.EqualFold(r, Rel) {
			return true
		}
	}
	return false
}

func resolve(pageURL, href string) string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}
