package diffview

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/cloudboy-jh/bentotui/theme"
)

type LineKind int

const (
	LineContext LineKind = iota
	LineAdd
	LineDelete
)

type InlineSpan struct {
	Text    string
	Changed bool
}

type DiffLine struct {
	Kind       LineKind
	OldNum     int
	NewNum     int
	Content    string
	InlineDiff []InlineSpan
}

type Hunk struct {
	Header   string
	OldStart int
	NewStart int
	Lines    []DiffLine
}

type FileDiff struct {
	OldName string
	NewName string
	Hunks   []Hunk
	Added   int
	Deleted int
}

type State struct {
	Files        []FileDiff
	FileIdx      int
	ScrollY      int
	Width        int
	Height       int
	ContextLines int
	Source       string
	CommitSHA    string
	LoadedForDir string
	SelectedPath string
}

type RenderResult struct {
	Lines    []string
	HunkRows []int
}

var hunkHeaderRE = regexp.MustCompile(`^@@\s+-(\d+)(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@(.*)$`)

func ParseUnifiedDiff(raw string) ([]FileDiff, error) {
	raw = strings.ReplaceAll(raw, "\r", "")
	lines := strings.Split(raw, "\n")
	files := make([]FileDiff, 0, 12)

	var cur *FileDiff
	var curHunk *Hunk

	flushHunk := func() {
		if cur != nil && curHunk != nil {
			cur.Hunks = append(cur.Hunks, *curHunk)
			curHunk = nil
		}
	}
	flushFile := func() {
		flushHunk()
		if cur != nil {
			applyInlineDiff(cur)
			files = append(files, *cur)
			cur = nil
		}
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushFile()
			oldName, newName := parseDiffGitHeader(line)
			cur = &FileDiff{OldName: oldName, NewName: newName}
		case strings.HasPrefix(line, "--- "):
			if cur != nil {
				cur.OldName = strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			}
		case strings.HasPrefix(line, "+++ "):
			if cur != nil {
				cur.NewName = strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			}
		case strings.HasPrefix(line, "@@ "):
			if cur == nil {
				continue
			}
			flushHunk()
			oldStart, newStart := parseHunkStarts(line)
			curHunk = &Hunk{Header: line, OldStart: oldStart, NewStart: newStart, Lines: make([]DiffLine, 0, 64)}
		case strings.HasPrefix(line, "\\ No newline at end of file"):
			if curHunk != nil {
				curHunk.Lines = append(curHunk.Lines, DiffLine{Kind: LineContext, Content: line})
			}
		default:
			if curHunk == nil {
				continue
			}
			if len(line) == 0 {
				curHunk.Lines = append(curHunk.Lines, DiffLine{Kind: LineContext, Content: ""})
				continue
			}
			prefix := line[0]
			text := line[1:]
			switch prefix {
			case ' ':
				curHunk.Lines = append(curHunk.Lines, DiffLine{Kind: LineContext, Content: text})
			case '+':
				cur.Added++
				curHunk.Lines = append(curHunk.Lines, DiffLine{Kind: LineAdd, Content: text})
			case '-':
				cur.Deleted++
				curHunk.Lines = append(curHunk.Lines, DiffLine{Kind: LineDelete, Content: text})
			}
		}
	}
	flushFile()

	for fi := range files {
		for hi := range files[fi].Hunks {
			oldNum := files[fi].Hunks[hi].OldStart
			newNum := files[fi].Hunks[hi].NewStart
			for li := range files[fi].Hunks[hi].Lines {
				dl := &files[fi].Hunks[hi].Lines[li]
				switch dl.Kind {
				case LineContext:
					dl.OldNum = oldNum
					dl.NewNum = newNum
					oldNum++
					newNum++
				case LineDelete:
					dl.OldNum = oldNum
					oldNum++
				case LineAdd:
					dl.NewNum = newNum
					newNum++
				}
			}
		}
	}

	return files, nil
}

func RenderFile(file FileDiff, width int, t theme.Theme, contextLines int) RenderResult {
	if contextLines < 0 {
		contextLines = 0
	}
	if width < 20 {
		width = 20
	}

	rows := make([]string, 0, 512)
	hunkRows := make([]int, 0, len(file.Hunks))

	fileName := displayName(file)
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Text.Accent)).Render(fileName)
	headerRight := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(fmt.Sprintf("+%d -%d", file.Added, file.Deleted))
	headerPad := max(1, width-lipgloss.Width(headerLeft)-lipgloss.Width(headerRight))
	header := headerLeft + strings.Repeat(" ", headerPad) + headerRight
	header = lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(lipgloss.Color(t.Surface.Panel)).
		Render(header)
	rows = append(rows, header)

	showLine := func(oldN, newN int, kind LineKind, content string, spans []InlineSpan) string {
		return renderCodeLine(oldN, newN, kind, content, spans, fileName, width, t)
	}

	for _, h := range file.Hunks {
		hunkRows = append(hunkRows, len(rows))
		hdr := lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.State.Info)).
			Background(lipgloss.Color(t.Surface.Panel)).
			Width(width).
			Padding(0, 1).
			Render(h.Header)
		rows = append(rows, hdr)

		firstChange, lastChange := changeBounds(h.Lines)
		for i, dl := range h.Lines {
			if dl.Kind == LineContext && firstChange >= 0 {
				if i < firstChange-contextLines || i > lastChange+contextLines {
					if i == firstChange-contextLines {
						hidden := (firstChange - contextLines) - 0
						if hidden > 0 {
							rows = append(rows, renderCollapsed(hidden, width, t))
						}
					}
					continue
				}
			}
			rows = append(rows, showLine(dl.OldNum, dl.NewNum, dl.Kind, dl.Content, dl.InlineDiff))
		}
	}

	return RenderResult{Lines: rows, HunkRows: hunkRows}
}

func parseDiffGitHeader(line string) (string, string) {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return "", ""
	}
	return parts[2], parts[3]
}

func parseHunkStarts(line string) (int, int) {
	m := hunkHeaderRE.FindStringSubmatch(line)
	if len(m) < 3 {
		return 0, 0
	}
	oldStart, _ := strconv.Atoi(m[1])
	newStart, _ := strconv.Atoi(m[2])
	return oldStart, newStart
}

func applyInlineDiff(file *FileDiff) {
	for hi := range file.Hunks {
		lines := file.Hunks[hi].Lines
		for i := 0; i < len(lines)-1; i++ {
			if lines[i].Kind != LineDelete || lines[i+1].Kind != LineAdd {
				continue
			}
			delSpans, addSpans := inlineWordDiff(lines[i].Content, lines[i+1].Content)
			file.Hunks[hi].Lines[i].InlineDiff = delSpans
			file.Hunks[hi].Lines[i+1].InlineDiff = addSpans
			i++
		}
	}
}

func inlineWordDiff(delLine, addLine string) ([]InlineSpan, []InlineSpan) {
	a := tokenize(delLine)
	b := tokenize(addLine)
	lcs := lcsMatrix(a, b)

	delSpans := make([]InlineSpan, 0, len(a))
	addSpans := make([]InlineSpan, 0, len(b))

	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			delSpans = append(delSpans, InlineSpan{Text: a[i]})
			addSpans = append(addSpans, InlineSpan{Text: b[j]})
			i++
			j++
			continue
		}
		if lcs[i+1][j] >= lcs[i][j+1] {
			delSpans = append(delSpans, InlineSpan{Text: a[i], Changed: true})
			i++
		} else {
			addSpans = append(addSpans, InlineSpan{Text: b[j], Changed: true})
			j++
		}
	}
	for i < len(a) {
		delSpans = append(delSpans, InlineSpan{Text: a[i], Changed: true})
		i++
	}
	for j < len(b) {
		addSpans = append(addSpans, InlineSpan{Text: b[j], Changed: true})
		j++
	}
	return mergeSpans(delSpans), mergeSpans(addSpans)
}

func tokenize(s string) []string {
	if s == "" {
		return []string{}
	}
	r := []rune(s)
	out := make([]string, 0, len(r))
	cur := strings.Builder{}
	mode := 0
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, ch := range r {
		next := charMode(ch)
		if mode == 0 {
			mode = next
			cur.WriteRune(ch)
			continue
		}
		if next != mode || next == 3 {
			flush()
			mode = next
		}
		cur.WriteRune(ch)
	}
	flush()
	return out
}

func charMode(r rune) int {
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
		return 1
	}
	if r == ' ' || r == '\t' {
		return 2
	}
	return 3
}

func lcsMatrix(a, b []string) [][]int {
	m := make([][]int, len(a)+1)
	for i := range m {
		m[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				m[i][j] = 1 + m[i+1][j+1]
			} else if m[i+1][j] >= m[i][j+1] {
				m[i][j] = m[i+1][j]
			} else {
				m[i][j] = m[i][j+1]
			}
		}
	}
	return m
}

func mergeSpans(spans []InlineSpan) []InlineSpan {
	if len(spans) == 0 {
		return spans
	}
	out := make([]InlineSpan, 0, len(spans))
	cur := spans[0]
	for i := 1; i < len(spans); i++ {
		if spans[i].Changed == cur.Changed {
			cur.Text += spans[i].Text
			continue
		}
		out = append(out, cur)
		cur = spans[i]
	}
	out = append(out, cur)
	return out
}

func changeBounds(lines []DiffLine) (int, int) {
	first := -1
	last := -1
	for i, line := range lines {
		if line.Kind == LineContext {
			continue
		}
		if first == -1 {
			first = i
		}
		last = i
	}
	return first, last
}

func renderCollapsed(n, width int, t theme.Theme) string {
	text := fmt.Sprintf("··· %d unchanged lines ···", n)
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Foreground(lipgloss.Color(t.Text.Muted)).
		Faint(true).
		Render(text)
}

func renderCodeLine(oldN, newN int, kind LineKind, content string, spans []InlineSpan, fileName string, width int, t theme.Theme) string {
	oldStr := lineNum(oldN)
	newStr := lineNum(newN)
	gutter := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Muted)).Render(fmt.Sprintf("%4s │ %4s ", oldStr, newStr))

	indicator := " "
	lineBG := ""
	inlineBG := ""
	indColor := lipgloss.Color(t.Text.Muted)

	switch kind {
	case LineAdd:
		indicator = "│"
		indColor = lipgloss.Color(t.State.Success)
		lineBG = blendHex(t.Surface.Canvas, t.State.Success, 0.18)
		inlineBG = blendHex(t.Surface.Canvas, t.State.Success, 0.38)
	case LineDelete:
		indicator = "│"
		indColor = lipgloss.Color(t.State.Danger)
		lineBG = blendHex(t.Surface.Canvas, t.State.Danger, 0.18)
		inlineBG = blendHex(t.Surface.Canvas, t.State.Danger, 0.38)
	}
	bar := lipgloss.NewStyle().Foreground(indColor).Render(indicator)

	codeW := max(1, width-lipgloss.Width(gutter)-2)
	code := renderCode(content, fileName, spans, inlineBG, kind, t)
	lineStyle := lipgloss.NewStyle().Width(codeW)
	if lineBG != "" {
		lineStyle = lineStyle.Background(lipgloss.Color(lineBG))
	}
	code = lineStyle.Render(code)
	return bar + gutter + code
}

func renderCode(content, fileName string, spans []InlineSpan, inlineBG string, kind LineKind, t theme.Theme) string {
	if len(spans) == 0 {
		return highlight(content, fileName, false, t)
	}
	b := strings.Builder{}
	base := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary))
	for _, sp := range spans {
		h := base.Render(sp.Text)
		if sp.Changed && (kind == LineAdd || kind == LineDelete) {
			h = lipgloss.NewStyle().Background(lipgloss.Color(inlineBG)).Bold(true).Render(h)
		}
		b.WriteString(h)
	}
	return b.String()
}

func highlight(content, fileName string, changed bool, t theme.Theme) string {
	if content == "" {
		return ""
	}
	lexer := lexers.Match(fileName)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	iter, err := lexer.Tokenise(nil, content)
	if err != nil {
		return content
	}
	b := strings.Builder{}
	for tok := iter(); tok != chroma.EOF; tok = iter() {
		style := tokenStyle(tok.Type, changed, t)
		b.WriteString(style.Render(tok.Value))
	}
	return b.String()
}

func tokenStyle(tt chroma.TokenType, changed bool, t theme.Theme) lipgloss.Style {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text.Primary))
	if changed {
		base = base.Bold(true)
	}
	switch {
	case tt.InCategory(chroma.Comment):
		return base.Foreground(lipgloss.Color(t.Text.Muted)).Italic(true)
	case tt.InCategory(chroma.Keyword):
		return base.Foreground(lipgloss.Color(t.Text.Accent)).Bold(true)
	case tt.InCategory(chroma.NameFunction):
		return base.Foreground(lipgloss.Color(t.State.Info))
	case tt.InCategory(chroma.NameClass), tt.InCategory(chroma.NameBuiltin):
		return base.Foreground(lipgloss.Color(t.Border.Focus))
	case tt.InCategory(chroma.LiteralString):
		return base.Foreground(lipgloss.Color(t.State.Success))
	case tt.InCategory(chroma.KeywordType), tt.InCategory(chroma.LiteralNumber):
		return base.Foreground(lipgloss.Color(t.State.Warning))
	default:
		return base
	}
}

func displayName(file FileDiff) string {
	name := strings.TrimPrefix(file.NewName, "b/")
	if name == "" || name == "/dev/null" {
		name = strings.TrimPrefix(file.OldName, "a/")
	}
	if name == "" {
		return "<unknown>"
	}
	return filepath.Clean(name)
}

func lineNum(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func blendHex(baseHex, tintHex string, ratio float64) string {
	br, bg, bb := parseHex(baseHex)
	tr, tg, tb := parseHex(tintHex)
	r := int(float64(br)*(1-ratio) + float64(tr)*ratio)
	g := int(float64(bg)*(1-ratio) + float64(tg)*ratio)
	b := int(float64(bb)*(1-ratio) + float64(tb)*ratio)
	if r < 0 {
		r = 0
	}
	if g < 0 {
		g = 0
	}
	if b < 0 {
		b = 0
	}
	if r > 255 {
		r = 255
	}
	if g > 255 {
		g = 255
	}
	if b > 255 {
		b = 255
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func parseHex(h string) (int, int, int) {
	h = strings.TrimPrefix(strings.TrimSpace(h), "#")
	if len(h) != 6 {
		return 0, 0, 0
	}
	r, _ := strconv.ParseInt(h[0:2], 16, 32)
	g, _ := strconv.ParseInt(h[2:4], 16, 32)
	b, _ := strconv.ParseInt(h[4:6], 16, 32)
	return int(r), int(g), int(b)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
