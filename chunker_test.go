package core

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

var smallParams = ChunkParams{Min: 40, Target: 120, Max: 200}

func mustChunk(t *testing.T, src string, p ChunkParams) []Chunk {
	t.Helper()
	cs, err := ChunkMarkdown(src, p)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

// para returns a paragraph of about n characters made of short sentences.
func para(word string, n int) string {
	var b strings.Builder
	for b.Len() < n {
		b.WriteString(word + " is a sentence. ")
	}
	return strings.TrimSpace(b.String())
}

func TestChunkSections(t *testing.T) {
	src := "Intro text before any heading, long enough to stand alone.\n\n" +
		"# A\n\nText under A, long enough to stand alone as a chunk.\n\n" +
		"## B\n\nText under B, long enough to stand alone as a chunk.\n\n" +
		"### C ###\n\nText under C, long enough to stand alone as a chunk.\n\n" +
		"## D\n\nText under D, long enough to stand alone as a chunk.\n"
	cs := mustChunk(t, src, smallParams)
	want := [][]string{nil, {"A"}, {"A", "B"}, {"A", "B", "C"}, {"A", "D"}}
	if len(cs) != len(want) {
		t.Fatalf("got %d chunks, want %d: %+v", len(cs), len(want), cs)
	}
	for i, c := range cs {
		if !slices.Equal(c.Section, want[i]) {
			t.Errorf("chunk %d section %q, want %q", i, c.Section, want[i])
		}
		if strings.Contains(c.Text, "#") {
			t.Errorf("chunk %d contains heading text: %q", i, c.Text)
		}
	}
}

func TestChunkMergesSmallBlocks(t *testing.T) {
	src := "One.\n\nTwo.\n\nThree.\n\nFour."
	cs := mustChunk(t, src, smallParams)
	if len(cs) != 1 || cs[0].Text != "One.\n\nTwo.\n\nThree.\n\nFour." {
		t.Fatalf("got %+v, want one merged chunk", cs)
	}
}

func TestChunkKeepsLargeBlocksApart(t *testing.T) {
	a, b, c := para("Alpha", 60), para("Beta", 60), para("Gamma", 60)
	cs := mustChunk(t, a+"\n\n"+b+"\n\n"+c, smallParams)
	if len(cs) != 3 || cs[0].Text != a || cs[1].Text != b || cs[2].Text != c {
		t.Fatalf("got %d chunks, want the three paragraphs as they are: %+v", len(cs), cs)
	}
}

func TestChunkNeverExceedsMax(t *testing.T) {
	long := para("Delta", 1000)
	unbroken := strings.Repeat("x", 450)
	cs := mustChunk(t, long+"\n\n"+unbroken, smallParams)
	if len(cs) < 5 {
		t.Fatalf("got %d chunks, want the oversize blocks split", len(cs))
	}
	var rebuilt []string
	for _, c := range cs {
		if n := utf8.RuneCountInString(c.Text); n > smallParams.Max {
			t.Fatalf("chunk of %d characters exceeds max %d", n, smallParams.Max)
		}
		rebuilt = append(rebuilt, c.Text)
	}
	joined := strings.Join(rebuilt, " ")
	if !strings.Contains(joined, "Delta is a sentence.") || strings.Count(joined, "x") != 450 {
		t.Fatal("split lost content")
	}
}

func TestChunkFencedCode(t *testing.T) {
	code := "```go\nfunc main() {\n\n\tprintln(\"blank line above stays inside\")\n}\n```"
	cs := mustChunk(t, "Before the code, long enough to stand alone as a chunk.\n\n"+code, smallParams)
	if len(cs) != 2 || cs[1].Text != code {
		t.Fatalf("fenced code not kept whole: %+v", cs)
	}

	var body []string
	for i := 0; i < 40; i++ {
		body = append(body, "line of code number "+strings.Repeat("x", i%7))
	}
	big := "~~~\n" + strings.Join(body, "\n") + "\n~~~"
	for _, c := range mustChunk(t, big, smallParams) {
		if !strings.HasPrefix(c.Text, "~~~\n") || !strings.HasSuffix(c.Text, "\n~~~") {
			t.Fatalf("oversize code piece not rewrapped in its fence: %q", c.Text)
		}
		if utf8.RuneCountInString(c.Text) > smallParams.Max {
			t.Fatalf("code piece exceeds max: %d", utf8.RuneCountInString(c.Text))
		}
	}
}

func TestChunkSplitsListsByLine(t *testing.T) {
	var items []string
	for i := 0; i < 30; i++ {
		items = append(items, "- list item with some words in it")
	}
	cs := mustChunk(t, strings.Join(items, "\n"), smallParams)
	for _, c := range cs {
		if !strings.HasPrefix(c.Text, "- ") {
			t.Fatalf("list piece does not start at an item: %q", c.Text)
		}
	}
}

func TestChunkNormalization(t *testing.T) {
	lf := mustChunk(t, "Café au lait.\n\nSecond line.", smallParams)
	crlf := mustChunk(t, "Café au lait.\r\n\r\nSecond line.   ", smallParams) // NFD, CRLF, trailing spaces
	if len(lf) != len(crlf) || lf[0].ID() != crlf[0].ID() {
		t.Fatalf("normalization differs:\n%q\n%q", lf, crlf)
	}
}

func TestChunkDeterministic(t *testing.T) {
	src := "# T\n\n" + para("Echo", 500) + "\n\n" + para("Foxtrot", 90)
	a, b := mustChunk(t, src, smallParams), mustChunk(t, src, smallParams)
	if !slices.EqualFunc(a, b, func(x, y Chunk) bool { return x.ID() == y.ID() && slices.Equal(x.Section, y.Section) }) {
		t.Fatal("same input gave different chunks")
	}
}

func TestChunkStableAcrossEdits(t *testing.T) {
	a, b, c := para("Alpha", 60), para("Beta", 60), para("Gamma", 60)
	before := mustChunk(t, a+"\n\n"+b+"\n\n"+c, smallParams)
	after := mustChunk(t, a+"\n\n"+strings.Replace(b, "Beta", "Bravo", 1)+"\n\n"+c, smallParams)
	if len(before) != 3 || len(after) != 3 {
		t.Fatalf("unexpected chunk counts %d, %d", len(before), len(after))
	}
	if before[0].ID() != after[0].ID() || before[2].ID() != after[2].ID() {
		t.Fatal("unchanged paragraphs changed ID")
	}
	if before[1].ID() == after[1].ID() {
		t.Fatal("edited paragraph kept its ID")
	}
}

func TestChunkParamsValidate(t *testing.T) {
	for _, p := range []ChunkParams{{0, 10, 20}, {30, 20, 40}, {10, 50, 40}} {
		if _, err := ChunkMarkdown("x", p); err == nil {
			t.Errorf("params %+v accepted", p)
		}
	}
	if err := DefaultChunkParams.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSentences(t *testing.T) {
	got := sentences("First one. \"Quoted!\" Then (a parenthetical.) Last? 文章です。次")
	want := []string{"First one.", "\"Quoted!\"", "Then (a parenthetical.)", "Last?", "文章です。", "次"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
