package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var appVersion = "0.5.5"

type UpdateCheckResult struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Version        string `json:"version,omitempty"`
	Available      bool   `json:"available"`
	Mandatory      bool   `json:"mandatory"`
	Platform       string `json:"platform"`
	FileName       string `json:"filename"`
	URL            string `json:"url"`
	SHA256         string `json:"sha256"`
	Size           int64  `json:"size"`
	PublishedAt    string `json:"published_at"`
}

func (a *App) CheckForUpdate() (*UpdateCheckResult, error) {
	if runtime.GOOS != "windows" {
		return &UpdateCheckResult{CurrentVersion: appVersion}, nil
	}

	a.mMu.Lock()
	serverURL := ""
	if a.mSettings != nil {
		serverURL = strings.TrimRight(a.mSettings.ServerURL, "/")
	}
	a.mMu.Unlock()
	if serverURL == "" {
		return nil, fmt.Errorf("server URL is not configured")
	}

	req, err := http.NewRequest("GET", serverURL+"/api/v1/client-updates/windows/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ProIdentity-Access/"+appVersion)

	c := &http.Client{Timeout: 15 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("check update: HTTP %d", res.StatusCode)
	}

	var m UpdateCheckResult
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return nil, err
	}
	if m.LatestVersion == "" {
		m.LatestVersion = m.Version
	}
	m.CurrentVersion = appVersion
	m.Available = m.LatestVersion != "" && versionGreater(m.LatestVersion, appVersion)
	return &m, nil
}

func versionGreater(a, b string) bool {
	ap := parseVersion(a)
	bp := parseVersion(b)
	for i := 0; i < len(ap) || i < len(bp); i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}
