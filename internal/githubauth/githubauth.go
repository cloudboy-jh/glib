package githubauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	StatusSignedOut = "SIGNED_OUT"
	StatusPending   = "PENDING"
	StatusAuth      = "AUTHORIZED"
	StatusExpired   = "EXPIRED"
)

type Repo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
}

type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type accessTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func StartDeviceFlow(clientID, scope string) (DeviceCode, error) {
	if strings.TrimSpace(clientID) == "" {
		return DeviceCode{}, fmt.Errorf("missing GitHub client id (set GLIB_GITHUB_CLIENT_ID)")
	}
	body := map[string]string{"client_id": clientID}
	if strings.TrimSpace(scope) != "" {
		body["scope"] = strings.TrimSpace(scope)
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/device/code", bytes.NewReader(payload))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DeviceCode{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return DeviceCode{}, fmt.Errorf("github device flow failed: %s", resp.Status)
	}
	var out DeviceCode
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return DeviceCode{}, err
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	return out, nil
}

func PollAccessToken(clientID, deviceCode string, intervalSeconds int) (string, error) {
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		payload, _ := json.Marshal(map[string]string{
			"client_id":   clientID,
			"device_code": deviceCode,
			"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		})
		req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", bytes.NewReader(payload))
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		var out accessTokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			_ = resp.Body.Close()
			return "", err
		}
		_ = resp.Body.Close()
		if out.AccessToken != "" {
			return out.AccessToken, nil
		}
		switch out.Error {
		case "authorization_pending":
			time.Sleep(time.Duration(intervalSeconds) * time.Second)
			continue
		case "slow_down":
			intervalSeconds += 2
			time.Sleep(time.Duration(intervalSeconds) * time.Second)
			continue
		case "expired_token", "access_denied":
			return "", fmt.Errorf("%s", out.Error)
		default:
			if out.ErrorDescription != "" {
				return "", fmt.Errorf("%s", out.ErrorDescription)
			}
			return "", fmt.Errorf("oauth failed: %s", out.Error)
		}
	}
	return "", fmt.Errorf("device flow timed out")
}

func ListRepos(token string, page, perPage int) ([]Repo, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	url := fmt.Sprintf("https://api.github.com/user/repos?sort=updated&per_page=%d&page=%d", perPage, page)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github repos failed: %s", resp.Status)
	}
	var repos []Repo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func tokenPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "glib")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "github_token"), nil
}

func SaveToken(token string) error {
	p, err := tokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(strings.TrimSpace(token)), 0o600)
}

func LoadToken() (string, error) {
	p, err := tokenPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func ClearToken() error {
	p, err := tokenPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
