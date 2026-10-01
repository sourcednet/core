package htmllink

import (
	"net/http"
	"testing"
)

const page = "https://example.test/guides/a.html"

func TestFromHTML(t *testing.T) {
	for _, tt := range []struct {
		body, want string
	}{
		{`<html><head><link rel="sourced-record" href="/.well-known/sourced/records/aa.json"></head></html>`,
			"https://example.test/.well-known/sourced/records/aa.json"},
		{`<head><link rel="stylesheet" href="/s.css"><link href="../r.json" rel="alternate sourced-record"/></head>`,
			"https://example.test/r.json"},
		{`<head><title>No link</title></head><body><link rel="sourced-record" href="/late.json"></body>`, ""},
		{`<head></head><link rel="sourced-record" href="/after-head.json">`, ""},
		{``, ""},
	} {
		if got := FromHTML([]byte(tt.body), page); got != tt.want {
			t.Errorf("FromHTML(%q) = %q, want %q", tt.body, got, tt.want)
		}
	}
}

func TestFromHeader(t *testing.T) {
	for _, tt := range []struct {
		values []string
		want   string
	}{
		{[]string{`</.well-known/sourced/records/aa.json>; rel="sourced-record"`}, "https://example.test/.well-known/sourced/records/aa.json"},
		{[]string{`</style.css>; rel=preload, </r.json>; rel=sourced-record`}, "https://example.test/r.json"},
		{[]string{`</a>; rel="preload"`, `<https://example.test/b.json>; type="x"; rel="next sourced-record"`}, "https://example.test/b.json"},
		{[]string{`</a>; rel="preload"`}, ""},
		{nil, ""},
	} {
		h := http.Header{}
		for _, v := range tt.values {
			h.Add("Link", v)
		}
		if got := FromHeader(h, page); got != tt.want {
			t.Errorf("FromHeader(%q) = %q, want %q", tt.values, got, tt.want)
		}
	}
}
