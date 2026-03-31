package piui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"github.com/cloudboy-jh/bentotui/registry/bricks/input"
	"github.com/cloudboy-jh/bentotui/theme"
	"github.com/cloudboy-jh/glib/internal/slash"
)

type Session struct {
	Input    *input.Model
	Viewport viewport.Model

	Messages []Message

	AutoScroll   bool
	Streaming    bool
	ToolRunning  bool
	Status       string
	CmdPrefix    bool
	SteerMode    bool
	ShowThinking bool
	ShowToolBody bool
	DebugEvents  bool

	SessionName   string
	SessionID     string
	ModelName     string
	ThinkingLevel string
	TokenTotal    int
	ContextWindow int
	ContextPct    int
	Cost          float64
	CommandHints  []string
	Commands      []SlashCommand
	SlashMatches  []int
	SlashCursor   int
	SlashQuery    string
	WidgetLines   []string

	activeAssistant int
	openTools       map[string]int
	activeExec      map[string]struct{}
	lastToolID      string
	anonToolSeq     int
	thinkingIdx     int

	Modal Modal

	spinner spinner
}

type SlashCommand struct {
	Name        string
	Description string
	Source      string
}

type SlashRow struct {
	Command     string
	Description string
	Selected    bool
}

func NewSession() *Session {
	inp := input.New()
	inp.SetPlaceholder("Message pi...")
	inp.Focus()
	vp := viewport.New()
	vp.SetContent("")
	return &Session{
		Input:           inp,
		Viewport:        vp,
		AutoScroll:      true,
		Status:          "idle",
		ShowThinking:    false,
		ShowToolBody:    false,
		DebugEvents:     false,
		activeAssistant: -1,
		thinkingIdx:     -1,
		openTools:       map[string]int{},
		activeExec:      map[string]struct{}{},
		spinner:         newSpinner(),
	}
}

func (s *Session) SetSize(width, height int) {
	s.Input.SetSize(max(20, width-8), 1)
	s.Viewport.SetWidth(max(1, width-4))
	s.Viewport.SetHeight(max(1, height-4))
}

func (s *Session) Refresh(t theme.Theme) {
	content := RenderHistory(s.Messages, max(8, s.Viewport.Width()), t)
	s.Viewport.SetContent(content)
	if s.AutoScroll {
		s.Viewport.GotoBottom()
	}
}

func (s *Session) InputLine() string {
	if s.SteerMode {
		return "steer> " + s.Input.Value()
	}
	return "> " + s.Input.Value()
}

func (s *Session) StartStreaming() {
	s.Streaming = true
	s.Status = "thinking"
	s.activeAssistant = -1
	s.AutoScroll = true
}

func (s *Session) AppendUserMessage(text string) {
	s.Messages = append(s.Messages, Message{Role: RoleUser, Text: text})
}

func (s *Session) StopStreaming() {
	s.finishActiveAssistant()
	s.Streaming = false
	s.resetToolActivity(false)
	s.clearThinkingBlock()
	if s.Status == "thinking" || s.Status == "aborting" {
		s.Status = "idle"
	}
}

func (s *Session) SpinnerFrame() string {
	if s.Busy() {
		return s.spinner.frame()
	}
	return ""
}

func (s *Session) Busy() bool {
	if s.Streaming {
		return true
	}
	if s.ToolRunning || len(s.activeExec) > 0 {
		return true
	}
	state := strings.TrimSpace(s.Status)
	return state == "retrying" || state == "compacting" || state == "aborting"
}

func (s *Session) TickSpinner() {
	s.spinner.tick()
}

func (s *Session) OpenToolByID(id string, name string, args string) int {
	if id == "" {
		s.anonToolSeq++
		id = fmt.Sprintf("anon-%d", s.anonToolSeq)
	}
	if idx, ok := s.openTools[id]; ok && idx >= 0 && idx < len(s.Messages) {
		if s.Messages[idx].ToolBlock != nil {
			s.Messages[idx].ToolBlock.Name = maxString(s.Messages[idx].ToolBlock.Name, name)
			if strings.TrimSpace(args) != "" {
				s.Messages[idx].ToolBlock.Args = args
			}
			return idx
		}
	}
	idx := len(s.Messages)
	s.Messages = append(s.Messages, Message{Role: RoleTool, ToolBlock: &ToolBlock{Name: name, Args: args, Collapsed: !s.ShowToolBody}})
	s.openTools[id] = idx
	s.lastToolID = id
	return idx
}

func (s *Session) StartToolExecution(id string, name string, args string) int {
	idx := s.OpenToolByID(id, name, args)
	if idx >= 0 && idx < len(s.Messages) && s.Messages[idx].ToolBlock != nil {
		s.Messages[idx].ToolBlock.Running = true
		s.Messages[idx].ToolBlock.Done = false
		s.Messages[idx].ToolBlock.ExitOK = false
		s.Messages[idx].ToolBlock.Collapsed = !s.ShowToolBody
	}
	if id != "" {
		s.activeExec[id] = struct{}{}
		s.lastToolID = id
	}
	s.ToolRunning = len(s.activeExec) > 0
	return idx
}

func (s *Session) CloseToolByID(id string, ok bool) {
	if id == "" {
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Role == RoleTool && s.Messages[i].ToolBlock != nil && !s.Messages[i].ToolBlock.Done {
				s.Messages[i].ToolBlock.Done = true
				s.Messages[i].ToolBlock.ExitOK = ok
				break
			}
		}
		s.ToolRunning = len(s.activeExec) > 0
		return
	}
	if idx, okMap := s.openTools[id]; okMap && idx >= 0 && idx < len(s.Messages) {
		if s.Messages[idx].ToolBlock != nil {
			s.Messages[idx].ToolBlock.Running = false
			s.Messages[idx].ToolBlock.Done = true
			s.Messages[idx].ToolBlock.ExitOK = ok
			s.Messages[idx].ToolBlock.Collapsed = !s.ShowToolBody
		}
	}
	delete(s.activeExec, id)
	s.ToolRunning = len(s.activeExec) > 0
}

func (s *Session) UpdateToolOutput(id string, out string) {
	out = strings.TrimSpace(out)
	if out == "" {
		return
	}
	if id != "" {
		if idx, ok := s.openTools[id]; ok && idx >= 0 && idx < len(s.Messages) {
			if s.Messages[idx].ToolBlock != nil {
				s.Messages[idx].ToolBlock.Output = out
			}
			s.lastToolID = id
			return
		}
	}
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == RoleTool && s.Messages[i].ToolBlock != nil && !s.Messages[i].ToolBlock.Done {
			s.Messages[i].ToolBlock.Output = out
			s.lastToolID = ""
			return
		}
	}
}

func (s *Session) UpsertThinking(text string) {
	if !s.ShowThinking {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if s.thinkingIdx >= 0 && s.thinkingIdx < len(s.Messages) {
		if tb := s.Messages[s.thinkingIdx].ToolBlock; tb != nil {
			tb.Output = text
			tb.Running = s.Streaming
			tb.Done = !s.Streaming
			return
		}
	}
	s.Messages = append(s.Messages, Message{Role: RoleTool, ToolBlock: &ToolBlock{Name: "thinking", Output: text, Running: s.Streaming, Collapsed: true, Kind: "thinking"}})
	s.thinkingIdx = len(s.Messages) - 1
}

func (s *Session) clearThinkingBlock() {
	if s.thinkingIdx >= 0 && s.thinkingIdx < len(s.Messages) {
		if tb := s.Messages[s.thinkingIdx].ToolBlock; tb != nil {
			tb.Running = false
			tb.Done = true
		}
	}
	s.thinkingIdx = -1
}

func (s *Session) resetToolActivity(markDone bool) {
	for id := range s.activeExec {
		if idx, ok := s.openTools[id]; ok && idx >= 0 && idx < len(s.Messages) {
			if tb := s.Messages[idx].ToolBlock; tb != nil {
				tb.Running = false
				if markDone {
					tb.Done = true
				}
			}
		}
	}
	s.activeExec = map[string]struct{}{}
	s.ToolRunning = false
}

func (s *Session) ToggleToolBody() {
	s.ShowToolBody = !s.ShowToolBody
	if s.lastToolID != "" {
		if idx, ok := s.openTools[s.lastToolID]; ok && idx >= 0 && idx < len(s.Messages) {
			if tb := s.Messages[idx].ToolBlock; tb != nil {
				tb.Collapsed = !s.ShowToolBody
			}
		}
		return
	}
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if tb := s.Messages[i].ToolBlock; tb != nil {
			tb.Collapsed = !s.ShowToolBody
			break
		}
	}
}

func (s *Session) ToggleThinking() {
	s.ShowThinking = !s.ShowThinking
	if !s.ShowThinking {
		s.clearThinkingBlock()
	}
}

func (s *Session) ApplyState(data map[string]any) {
	if data == nil {
		return
	}
	if model := asMap(data["model"]); len(model) > 0 {
		s.ModelName = nonEmptyString(asString(model["name"]), asString(model["id"]), s.ModelName)
		s.ContextWindow = asInt(model["contextWindow"], s.ContextWindow)
	}
	s.ThinkingLevel = nonEmptyString(asString(data["thinkingLevel"]), s.ThinkingLevel)
	s.SessionName = nonEmptyString(asString(data["sessionName"]), s.SessionName)
	s.SessionID = nonEmptyString(asString(data["sessionId"]), s.SessionID)
	s.recomputeContextPct()
}

func (s *Session) ApplyStats(data map[string]any) {
	if data == nil {
		return
	}
	tokens := asMap(data["tokens"])
	s.TokenTotal = asInt(tokens["total"], s.TokenTotal)
	s.Cost = asFloat(data["cost"], s.Cost)
	s.recomputeContextPct()
}

func (s *Session) ApplyCommands(data map[string]any) {
	raw := []any{}
	if data != nil {
		if parsed, ok := data["commands"].([]any); ok {
			raw = parsed
		}
	}
	hints := make([]string, 0, 3)
	for _, v := range raw {
		cmd := asMap(v)
		name := strings.TrimSpace(asString(cmd["name"]))
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		hints = append(hints, name)
		if len(hints) >= 3 {
			break
		}
	}
	if len(hints) > 0 {
		s.CommandHints = hints
	}

	cmds := make([]SlashCommand, 0, len(raw))
	for _, v := range raw {
		cmd := asMap(v)
		name := strings.TrimSpace(asString(cmd["name"]))
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		desc := strings.TrimSpace(asString(cmd["description"]))
		source := strings.TrimSpace(asString(cmd["source"]))
		cmds = append(cmds, SlashCommand{Name: name, Description: desc, Source: source})
	}
	cmds = mergeBuiltinSlashCommands(cmds)
	s.Commands = cmds
	if len(hints) == 0 {
		for _, c := range cmds {
			hints = append(hints, c.Name)
			if len(hints) >= 3 {
				break
			}
		}
	}
	if len(hints) > 0 {
		s.CommandHints = hints
	}
	s.UpdateSlashQuery(s.Input.Value())
}

func (s *Session) HeaderContextLine() string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(s.ModelName) != "" {
		parts = append(parts, s.ModelName)
	}
	if strings.TrimSpace(s.ThinkingLevel) != "" {
		parts = append(parts, "think "+s.ThinkingLevel)
	}
	if s.ContextPct > 0 && s.ContextWindow > 0 {
		parts = append(parts, fmt.Sprintf("ctx %d%%", s.ContextPct))
	}
	if s.TokenTotal > 0 {
		parts = append(parts, fmt.Sprintf("tok %s", formatThousands(s.TokenTotal)))
	}
	if strings.TrimSpace(s.SessionName) != "" {
		parts = append(parts, "session "+s.SessionName)
	} else if strings.TrimSpace(s.SessionID) != "" {
		parts = append(parts, "session "+shortID(s.SessionID))
	}
	return strings.Join(parts, "  •  ")
}

func (s *Session) HeaderCommandsLine() string {
	if len(s.CommandHints) > 0 {
		return strings.Join(s.CommandHints, "  ")
	}
	return "/models  /new  /help"
}

func (s *Session) HeaderRightLine() string {
	meta := strings.TrimSpace(s.HeaderContextLine())
	cmds := strings.TrimSpace(s.HeaderCommandsLine())
	if meta != "" {
		return meta
	}
	return cmds
}

func (s *Session) UpdateSlashQuery(input string) {
	if !strings.HasPrefix(input, "/") {
		s.SlashQuery = ""
		s.SlashMatches = nil
		s.SlashCursor = 0
		return
	}
	q := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(input)), "/")
	s.SlashQuery = q
	if len(s.Commands) == 0 {
		s.SlashMatches = nil
		s.SlashCursor = 0
		return
	}
	matches := make([]int, 0, len(s.Commands))
	prefix := make([]int, 0, len(s.Commands))
	contains := make([]int, 0, len(s.Commands))
	for i, c := range s.Commands {
		name := strings.TrimPrefix(strings.ToLower(c.Name), "/")
		if q == "" {
			matches = append(matches, i)
			continue
		}
		if strings.HasPrefix(name, q) {
			prefix = append(prefix, i)
			continue
		}
		if strings.Contains(name, q) {
			contains = append(contains, i)
		}
	}
	if q != "" {
		matches = append(matches, prefix...)
		matches = append(matches, contains...)
	}
	s.SlashMatches = matches
	if len(s.SlashMatches) == 0 {
		s.SlashCursor = 0
	} else {
		s.SlashCursor = clamp(s.SlashCursor, 0, len(s.SlashMatches)-1)
	}
}

func (s *Session) SlashActive() bool {
	return strings.HasPrefix(s.Input.Value(), "/")
}

func (s *Session) MoveSlashCursor(delta int) {
	if len(s.SlashMatches) == 0 {
		s.SlashCursor = 0
		return
	}
	s.SlashCursor = clamp(s.SlashCursor+delta, 0, len(s.SlashMatches)-1)
}

func (s *Session) SelectedSlashCommand() (SlashCommand, bool) {
	if len(s.SlashMatches) == 0 {
		return SlashCommand{}, false
	}
	idx := s.SlashMatches[clamp(s.SlashCursor, 0, len(s.SlashMatches)-1)]
	if idx < 0 || idx >= len(s.Commands) {
		return SlashCommand{}, false
	}
	return s.Commands[idx], true
}

func (s *Session) HasExactSlashCommand(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	v = strings.ToLower(v)
	for _, cmd := range s.Commands {
		if strings.ToLower(strings.TrimSpace(cmd.Name)) == v {
			return true
		}
	}
	return false
}

func (s *Session) AutocompleteSlashInput() bool {
	cmd, ok := s.SelectedSlashCommand()
	if !ok {
		return false
	}
	s.Input.SetValue(cmd.Name)
	s.UpdateSlashQuery(cmd.Name)
	return true
}

func (s *Session) SlashRows(limit int) []SlashRow {
	if limit <= 0 {
		limit = 8
	}
	if len(s.SlashMatches) == 0 {
		return nil
	}
	start := windowStart(s.SlashCursor, limit, len(s.SlashMatches))
	end := min(len(s.SlashMatches), start+limit)
	rows := make([]SlashRow, 0, end-start)
	for i := start; i < end; i++ {
		idx := s.SlashMatches[i]
		if idx < 0 || idx >= len(s.Commands) {
			continue
		}
		cmd := s.Commands[idx]
		desc := cmd.Description
		if strings.TrimSpace(desc) == "" {
			desc = cmd.Source
		}
		rows = append(rows, SlashRow{Command: cmd.Name, Description: desc, Selected: i == s.SlashCursor})
	}
	return rows
}

func (s *Session) recomputeContextPct() {
	if s.ContextWindow <= 0 || s.TokenTotal <= 0 {
		s.ContextPct = 0
		return
	}
	pct := int((float64(s.TokenTotal) / float64(s.ContextWindow)) * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 999 {
		pct = 999
	}
	s.ContextPct = pct
}

func nonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func asInt(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return parsed
		}
	}
	return fallback
}

func asFloat(v any, fallback float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func shortID(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= 8 {
		return v
	}
	return v[:8]
}

func formatThousands(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	buf := make([]byte, 0, len(s)+len(s)/3)
	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}
	buf = append(buf, s[:rem]...)
	for i := rem; i < len(s); i += 3 {
		buf = append(buf, ',')
		buf = append(buf, s[i:i+3]...)
	}
	return string(buf)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func windowStart(cursor, viewRows, total int) int {
	if total <= 0 || viewRows >= total {
		return 0
	}
	if cursor < viewRows {
		return 0
	}
	if cursor >= total-viewRows {
		return total - viewRows
	}
	return cursor - viewRows/2
}

func mergeBuiltinSlashCommands(existing []SlashCommand) []SlashCommand {
	b := slash.Builtin()
	builtin := make([]SlashCommand, 0, len(b))
	for _, c := range b {
		builtin = append(builtin, SlashCommand{Name: c.Name, Description: c.Description, Source: "builtin"})
	}
	out := make([]SlashCommand, 0, len(existing)+len(builtin))
	seen := map[string]struct{}{}
	for _, c := range existing {
		key := strings.ToLower(strings.TrimSpace(c.Name))
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	for _, c := range builtin {
		key := strings.ToLower(strings.TrimSpace(c.Name))
		if _, ok := seen[key]; ok {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (s *Session) ensureActiveAssistant() int {
	if s.activeAssistant >= 0 && s.activeAssistant < len(s.Messages) {
		if s.Messages[s.activeAssistant].Role == RoleAssistant {
			return s.activeAssistant
		}
	}
	s.Messages = append(s.Messages, Message{Role: RoleAssistant, Streaming: true})
	s.activeAssistant = len(s.Messages) - 1
	return s.activeAssistant
}

func (s *Session) finishActiveAssistant() {
	if s.activeAssistant >= 0 && s.activeAssistant < len(s.Messages) {
		s.Messages[s.activeAssistant].Streaming = false
	}
	s.activeAssistant = -1
}
