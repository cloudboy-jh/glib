package app

import (
	"strings"

	"glib/internal/config"
)

type settingsModel struct {
	values config.Settings
}

func loadSettingsModel() (settingsModel, error) {
	v, err := config.LoadSettings()
	if err != nil {
		return settingsModel{}, err
	}
	return settingsModel{values: v}, nil
}

func (s *settingsModel) Theme() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.values.Theme)
}

func (s *settingsModel) Model() string {
	if s == nil {
		return ""
	}
	if v := strings.TrimSpace(s.values.ModelID); v != "" {
		return v
	}
	return strings.TrimSpace(s.values.Model)
}

func (s *settingsModel) ModelProvider() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.values.ModelProvider)
}

func (s *settingsModel) SetTheme(themeName string) error {
	if s == nil {
		return nil
	}
	themeName = strings.TrimSpace(themeName)
	if s.values.Theme == themeName {
		return nil
	}
	s.values.Theme = themeName
	return config.SaveSettings(s.values)
}

func (s *settingsModel) SetModel(provider, modelID string) error {
	if s == nil {
		return nil
	}
	provider = strings.TrimSpace(provider)
	modelID = strings.TrimSpace(modelID)
	if s.values.ModelID == modelID && s.values.ModelProvider == provider {
		return nil
	}
	s.values.Model = modelID
	s.values.ModelID = modelID
	s.values.ModelProvider = provider
	return config.SaveSettings(s.values)
}
