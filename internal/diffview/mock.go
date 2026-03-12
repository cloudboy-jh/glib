package diffview

func MockFiles() []FileDiff {
	return []FileDiff{
		{
			OldName: "a/internal/diffview/renderer.go",
			NewName: "b/internal/diffview/renderer.go",
			Added:   18,
			Deleted: 6,
			Hunks: []Hunk{
				{
					Header:   "@@ -42,12 +42,24 @@ func renderLine(line DiffLine) string {",
					OldStart: 42,
					NewStart: 42,
					Lines: []DiffLine{
						{Kind: LineContext, OldNum: 42, NewNum: 42, Content: "func renderLine(line DiffLine) string {"},
						{Kind: LineContext, OldNum: 43, NewNum: 43, Content: "\tbase := lipgloss.NewStyle()"},
						{Kind: LineDelete, OldNum: 44, Content: "\treturn base.Render(line.Content)", InlineDiff: []InlineSpan{{Text: "\treturn base.Render(line.Content)", Changed: true}}},
						{Kind: LineAdd, NewNum: 44, Content: "\tcode := highlightTokens(line.Content)", InlineDiff: []InlineSpan{{Text: "\tcode := highlightTokens(line.Content)", Changed: true}}},
						{Kind: LineAdd, NewNum: 45, Content: "\treturn base.Background(lineTint(line.Kind)).Render(code)", InlineDiff: []InlineSpan{{Text: "\treturn base.Background(lineTint(line.Kind)).Render(code)", Changed: true}}},
						{Kind: LineContext, OldNum: 45, NewNum: 46, Content: "}"},
					},
				},
			},
		},
		{
			OldName: "a/internal/app/app.go",
			NewName: "b/internal/app/app.go",
			Added:   9,
			Deleted: 3,
			Hunks: []Hunk{
				{
					Header:   "@@ -302,7 +302,11 @@ func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {",
					OldStart: 302,
					NewStart: 302,
					Lines: []DiffLine{
						{Kind: LineContext, OldNum: 302, NewNum: 302, Content: "case \"d\":"},
						{Kind: LineContext, OldNum: 303, NewNum: 303, Content: "\tm.mode = modeDiff"},
						{Kind: LineDelete, OldNum: 304, Content: "\treturn m, m.refreshDiffCmd(\"\", \"\", \"\")", InlineDiff: []InlineSpan{{Text: "\treturn m, m.refreshDiffCmd(\"\", \"\", \"\")", Changed: true}}},
						{Kind: LineAdd, NewNum: 304, Content: "\tif useMockViews {", InlineDiff: []InlineSpan{{Text: "\tif useMockViews {", Changed: true}}},
						{Kind: LineAdd, NewNum: 305, Content: "\t\tm.diff.Files = diffview.MockFiles()", InlineDiff: []InlineSpan{{Text: "\t\tm.diff.Files = diffview.MockFiles()", Changed: true}}},
						{Kind: LineAdd, NewNum: 306, Content: "\t\treturn m, nil", InlineDiff: []InlineSpan{{Text: "\t\treturn m, nil", Changed: true}}},
						{Kind: LineAdd, NewNum: 307, Content: "\t}", InlineDiff: []InlineSpan{{Text: "\t}", Changed: true}}},
						{Kind: LineContext, OldNum: 305, NewNum: 308, Content: "\treturn m, m.refreshDiffCmd(\"\", \"\", \"\")"},
					},
				},
			},
		},
	}
}
