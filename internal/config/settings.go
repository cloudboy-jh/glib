package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	Theme         string `json:"theme,omitempty"`
	Model         string `json:"model,omitempty"`
	ModelID       string `json:"modelId,omitempty"`
	ModelProvider string `json:"modelProvider,omitempty"`
	RecentGitHub  []RecentGitHubRepo `json:"recentGithub,omitempty"`
	RecentLocal   []string           `json:"recentLocal,omitempty"`
}

type RecentGitHubRepo struct {
	FullName string `json:"fullName"`
	CloneURL string `json:"cloneUrl,omitempty"`
	Private  bool   `json:"private,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{}
}

func LoadSettings() (Settings, error) {
	p, err := settingsPath()
	if err != nil {
		return DefaultSettings(), err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return DefaultSettings(), err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return DefaultSettings(), nil
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return DefaultSettings(), err
	}
	return normalize(s), nil
}

func SaveSettings(s Settings) error {
	p, err := settingsPath()
	if err != nil {
		return err
	}
	s = normalize(s)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func settingsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "glib")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func normalize(s Settings) Settings {
	s.Theme = strings.TrimSpace(s.Theme)
	s.Model = strings.TrimSpace(s.Model)
	s.ModelID = strings.TrimSpace(s.ModelID)
	s.ModelProvider = strings.TrimSpace(s.ModelProvider)
	if s.ModelID == "" && s.Model != "" {
		s.ModelID = s.Model
	}
	if s.Model == "" && s.ModelID != "" {
		s.Model = s.ModelID
	}
	s.RecentLocal = normalizeRecentLocal(s.RecentLocal)
	s.RecentGitHub = normalizeRecentGitHub(s.RecentGitHub)
	return s
}

func normalizeRecentLocal(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
		if len(out) >= 20 {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeRecentGitHub(items []RecentGitHubRepo) []RecentGitHubRepo {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]RecentGitHubRepo, 0, len(items))
	for _, item := range items {
		full := strings.TrimSpace(item.FullName)
		if full == "" {
			continue
		}
		if _, ok := seen[full]; ok {
			continue
		}
		seen[full] = struct{}{}
		out = append(out, RecentGitHubRepo{
			FullName: full,
			CloneURL: strings.TrimSpace(item.CloneURL),
			Private:  item.Private,
		})
		if len(out) >= 20 {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
