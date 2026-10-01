package core

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Chunk is a passage of canonical text and the heading path it sits under.
type Chunk struct {
	Text    string
	Section []string
}

// ID returns the chunk's content-addressed ID.
func (c Chunk) ID() string { return ChunkID(c.Text) }

// ChunkParams are the chunker's size rules, in Unicode characters so they
// don't depend on any model's tokenizer. They are parameters, not constants,
// because they are what the benchmark experiments tune.
type ChunkParams struct {
	Min    int `json:"min"`    // blocks shorter than this are merged with neighbors
	Target int `json:"target"` // merging never grows a chunk past this
	Max    int `json:"max"`    // no chunk is ever longer than this
}

// DefaultChunkParams are the spec's values. Smaller chunks keep citations
// valid longer and answer queries in fewer tokens (benchmark, Oct 1).
var DefaultChunkParams = ChunkParams{Min: 150, Target: 800, Max: 2000}

// Validate checks that 0 < Min <= Target <= Max.
func (p ChunkParams) Validate() error {
	if p.Min <= 0 || p.Min > p.Target || p.Target > p.Max {
		return fmt.Errorf("chunk params: want 0 < min <= target <= max, got %d/%d/%d", p.Min, p.Target, p.Max)
	}
	return nil
}

// NormalizeText converts text to canonical form: LF line endings and Unicode NFC.
func NormalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return norm.NFC.String(s)
}

// ChunkMarkdown splits a Markdown page into chunks following the spec:
//
//   - Boundaries follow block structure: paragraphs, lists, tables, fenced
//     code, quotes. A block is split only if it exceeds p.Max, between
//     sentences (or lines, for code, lists, tables, and quotes).
//   - Consecutive blocks are merged while either side is shorter than p.Min
//     and the result stays within p.Target.
//   - ATX headings are not chunk text. They set the chunk's Section, and a
//     chunk never spans two sections.
//
// The same input and parameters always give the same chunks, so passages
// that don't change keep their IDs across revisions. Setext headings and
// indented code blocks are treated as plain paragraphs.
func ChunkMarkdown(src string, p ChunkParams) ([]Chunk, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	k := packer{p: p}
	var path []heading
	for _, b := range parseBlocks(NormalizeText(src)) {
		if b.kind != blockHeading {
			k.add(b)
			continue
		}
		for len(path) > 0 && path[len(path)-1].level >= b.level {
			path = path[:len(path)-1]
		}
		path = append(path, heading{level: b.level, title: b.text})
		k.flush()
		k.section = k.section[:0:0]
		for _, h := range path {
			k.section = append(k.section, h.title)
		}
	}
	k.flush()
	return k.out, nil
}

type heading struct {
	level int
	title string
}

type blockKind int

const (
	blockText blockKind = iota
	blockFence
	blockHeading
)

type block struct {
	kind  blockKind
	text  string // full block text; the title for headings
	level int    // heading level

	// Fenced code only: the fence lines and the lines between them.
	open, close string
	body        []string
}

// parseBlocks splits normalized Markdown into blocks. Trailing whitespace is
// dropped from every line.
func parseBlocks(src string) []block {
	lines := strings.Split(src, "\n")
	var blocks []block
	var para []string
	flush := func() {
		if len(para) > 0 {
			blocks = append(blocks, block{kind: blockText, text: strings.Join(para, "\n")})
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		if marker, n, ok := fenceOpen(line); ok {
			flush()
			b := block{kind: blockFence, open: line}
			for i++; i < len(lines); i++ {
				l := strings.TrimRight(lines[i], " \t")
				if fenceCloses(l, marker, n) {
					b.close = l
					break
				}
				b.body = append(b.body, l)
			}
			b.text = wrapFence(b.open, b.close, strings.Join(b.body, "\n"))
			blocks = append(blocks, b)
			continue
		}
		if line == "" {
			flush()
			continue
		}
		if level, title, ok := atxHeading(line); ok {
			flush()
			blocks = append(blocks, block{kind: blockHeading, level: level, text: title})
			continue
		}
		para = append(para, line)
	}
	flush()
	return blocks
}

// trimIndent strips up to three leading spaces. It reports false for four or
// more, which Markdown treats as indented code.
func trimIndent(line string) (string, bool) {
	n := len(line) - len(strings.TrimLeft(line, " "))
	if n > 3 {
		return line, false
	}
	return line[n:], true
}

func fenceOpen(line string) (marker byte, n int, ok bool) {
	s, ok := trimIndent(line)
	if !ok || len(s) < 3 || (s[0] != '`' && s[0] != '~') {
		return 0, 0, false
	}
	n = len(s) - len(strings.TrimLeft(s, s[:1]))
	if n < 3 || (s[0] == '`' && strings.Contains(s[n:], "`")) {
		return 0, 0, false
	}
	return s[0], n, true
}

func fenceCloses(line string, marker byte, n int) bool {
	s, ok := trimIndent(line)
	if !ok {
		return false
	}
	run := len(s) - len(strings.TrimLeft(s, string(marker)))
	return run >= n && strings.TrimSpace(s[run:]) == ""
}

func wrapFence(open, close, body string) string {
	s := open + "\n" + body
	if close != "" {
		s += "\n" + close
	}
	return s
}

func atxHeading(line string) (level int, title string, ok bool) {
	s, ok := trimIndent(line)
	if !ok {
		return 0, "", false
	}
	level = len(s) - len(strings.TrimLeft(s, "#"))
	if level < 1 || level > 6 {
		return 0, "", false
	}
	rest := s[level:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return 0, "", false
	}
	title = strings.TrimSpace(rest)
	// Drop an optional closing sequence of #s.
	if t := strings.TrimRight(title, "#"); t != title && (t == "" || strings.HasSuffix(t, " ") || strings.HasSuffix(t, "\t")) {
		title = strings.TrimSpace(t)
	}
	return level, title, true
}

// packer merges blocks into chunks within one section at a time.
type packer struct {
	p       ChunkParams
	section []string
	cur     []string
	curLen  int
	out     []Chunk
}

func (k *packer) add(b block) {
	n := utf8.RuneCountInString(b.text)
	switch {
	case n > k.p.Max:
		k.flush()
		for _, piece := range splitOversize(b, k.p) {
			k.emit(piece)
		}
	case len(k.cur) == 0:
		k.cur, k.curLen = []string{b.text}, n
	case k.curLen+2+n <= k.p.Target && (k.curLen < k.p.Min || n < k.p.Min):
		k.cur = append(k.cur, b.text)
		k.curLen += 2 + n
	default:
		k.flush()
		k.cur, k.curLen = []string{b.text}, n
	}
}

func (k *packer) flush() {
	if len(k.cur) > 0 {
		k.emit(strings.Join(k.cur, "\n\n"))
		k.cur, k.curLen = nil, 0
	}
}

func (k *packer) emit(text string) {
	var section []string
	if len(k.section) > 0 {
		section = slices.Clone(k.section)
	}
	k.out = append(k.out, Chunk{Text: text, Section: section})
}

// splitOversize splits a block longer than p.Max into pieces no longer than
// p.Max, aiming for p.Target.
func splitOversize(b block, p ChunkParams) []string {
	if b.kind == blockFence {
		// Split between lines and rewrap each piece in the fence, so every
		// piece still renders as code.
		overhead := utf8.RuneCountInString(wrapFence(b.open, b.close, ""))
		if overhead < p.Max {
			target := max(1, p.Target-overhead)
			pieces := pack(b.body, "\n", target, p.Max-overhead)
			for i, piece := range pieces {
				pieces[i] = wrapFence(b.open, b.close, piece)
			}
			return pieces
		}
		return hardSplit(b.text, p.Max)
	}
	if lineStructured(b.text) {
		return pack(strings.Split(b.text, "\n"), "\n", p.Target, p.Max)
	}
	return pack(sentences(b.text), " ", p.Target, p.Max)
}

// pack greedily joins units with sep into pieces of at most target
// characters. A unit longer than target gets a piece of its own, and a unit
// longer than max is split hard.
func pack(units []string, sep string, target, max int) []string {
	var out, cur []string
	curLen := 0
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, sep))
			cur, curLen = nil, 0
		}
	}
	sepLen := utf8.RuneCountInString(sep)
	for _, u := range units {
		n := utf8.RuneCountInString(u)
		if n > max {
			flush()
			out = append(out, hardSplit(u, max)...)
			continue
		}
		if len(cur) > 0 && curLen+sepLen+n > target {
			flush()
		}
		if len(cur) == 0 {
			curLen = n
		} else {
			curLen += sepLen + n
		}
		cur = append(cur, u)
	}
	flush()
	return out
}

// hardSplit cuts s into pieces of at most max characters. It is the last
// resort for text with no sentence or line boundaries.
func hardSplit(s string, max int) []string {
	var out []string
	r := []rune(s)
	for len(r) > max {
		out = append(out, string(r[:max]))
		r = r[max:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
	}
	return out
}

// lineStructured reports whether a multi-line block is a list, table, or
// quote, which split between lines rather than sentences.
func lineStructured(s string) bool {
	first, rest, ok := strings.Cut(s, "\n")
	if !ok || rest == "" {
		return false
	}
	t := strings.TrimLeft(first, " ")
	switch {
	case strings.HasPrefix(t, "|"), strings.HasPrefix(t, ">"),
		strings.HasPrefix(t, "- "), strings.HasPrefix(t, "* "), strings.HasPrefix(t, "+ "):
		return true
	}
	digits := len(t) - len(strings.TrimLeft(t, "0123456789"))
	return digits > 0 && (strings.HasPrefix(t[digits:], ". ") || strings.HasPrefix(t[digits:], ") "))
}

// SplitSentences splits prose into sentences: after ., !, or ? (and any
// closing quotes or brackets) followed by whitespace, and after CJK
// sentence-ending marks. The chunker uses it to split oversize paragraphs.
func SplitSentences(s string) []string { return sentences(s) }

func sentences(s string) []string {
	var out []string
	r := []rune(s)
	start := 0
	add := func(end int) {
		if t := strings.TrimSpace(string(r[start:end])); t != "" {
			out = append(out, t)
		}
		start = end
	}
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '。', '！', '？':
			add(i + 1)
		case '.', '!', '?':
			j := i + 1
			for j < len(r) && strings.ContainsRune("\"')]”’»", r[j]) {
				j++
			}
			if j < len(r) && unicode.IsSpace(r[j]) {
				add(j)
				i = j
			}
		}
	}
	add(len(r))
	return out
}
