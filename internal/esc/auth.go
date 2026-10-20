package esc

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const defaultAPI = "https://api.pulumi.com"

type backendSource int

const (
	backendNone backendSource = iota
	backendEnv
	backendProject
	backendCurrent
	backendDefault
)

type accountEntry struct {
	AccessToken string `json:"accessToken"`
	Username    string `json:"username"`
}

type credentialsFile struct {
	Current      string                  `json:"current"`
	AccessTokens map[string]string       `json:"accessTokens"`
	Accounts     map[string]accountEntry `json:"accounts"`
}

type backendConfig struct {
	DefaultOrg string `json:"defaultOrg"`
}

type pulumiUserConfig struct {
	Backends map[string]backendConfig `json:"backends"`
}

type authSession struct {
	BackendURL string
	Token      string
	creds      *credentialsFile
	config     *pulumiUserConfig
}

func discover(getenv func(string) string, projectBackend string) (authSession, *Failure) {
	getenv = orGetenv(getenv)
	creds, _, _, _ := readCredentials(credentialsPath(getenv))
	backend, source := chooseBackend(getenv("PULUMI_BACKEND_URL"), strings.TrimSpace(projectBackend), creds)
	if backend == "" {
		backend = defaultAPI
		source = backendDefault
	}
	if source == backendProject {
		return projectSession(getenv, backend, creds)
	}
	token := firstToken(getenv("PULUMI_ACCESS_TOKEN"), tokenFor(creds, backend))
	if token == "" {
		return authSession{}, &Failure{Category: CategoryNotLoggedIn}
	}
	if fail := validateBackend(backend); fail != nil {
		return authSession{}, fail
	}
	return authSession{
		BackendURL: backend,
		Token:      token,
		creds:      creds,
		config:     readUserConfig(getenv),
	}, nil
}

func projectSession(getenv func(string) string, backend string, creds *credentialsFile) (authSession, *Failure) {
	saved := tokenFor(creds, backend)
	if saved == "" {
		return authSession{}, &Failure{Category: CategoryUnsupportedBackend}
	}
	if fail := validateBackend(backend); fail != nil {
		return authSession{}, fail
	}
	return authSession{
		BackendURL: backend,
		Token:      saved,
		creds:      creds,
		config:     readUserConfig(getenv),
	}, nil
}

func chooseBackend(envURL, projectURL string, creds *credentialsFile) (string, backendSource) {
	switch {
	case envURL != "":
		return envURL, backendEnv
	case projectURL != "":
		return projectURL, backendProject
	case creds != nil && creds.Current != "":
		return creds.Current, backendCurrent
	default:
		return "", backendNone
	}
}

func firstToken(envToken, saved string) string {
	if envToken != "" {
		return envToken
	}
	return saved
}

func (a authSession) orgFor(ref Ref, fromStack bool, stackName string, getenv func(string) string) (string, *Failure) {
	if ref.Org != "" {
		return ref.Org, nil
	}
	if fromStack {
		if org := orgFromStack(stackName); org != "" {
			return org, nil
		}
	}
	return a.defaultOrg(orGetenv(getenv))
}

func (a authSession) defaultOrg(getenv func(string) string) (string, *Failure) {
	if org := getenv("PULUMI_DEFAULT_ORGANIZATION"); org != "" {
		return org, nil
	}
	if org := configuredOrg(a.config, a.BackendURL); org != "" {
		return org, nil
	}
	if org := accountUsername(a.creds, a.BackendURL); org != "" {
		return org, nil
	}
	return "", &Failure{Category: CategoryNotLoggedIn}
}

func orgFromStack(name string) string {
	trimmed := strings.TrimSpace(name)
	org, rest, ok := strings.Cut(trimmed, "/")
	if !ok || org == "" || rest == "" {
		return ""
	}
	return org
}

func configuredOrg(cfg *pulumiUserConfig, backend string) string {
	if cfg == nil {
		return ""
	}
	for _, key := range urlKeys(backend) {
		if entry, ok := cfg.Backends[key]; ok && entry.DefaultOrg != "" {
			return entry.DefaultOrg
		}
	}
	return ""
}

func accountUsername(creds *credentialsFile, backend string) string {
	if creds == nil {
		return ""
	}
	for _, key := range urlKeys(backend) {
		if entry, ok := creds.Accounts[key]; ok && entry.Username != "" {
			return entry.Username
		}
	}
	return ""
}

func tokenFor(creds *credentialsFile, backend string) string {
	if creds == nil {
		return ""
	}
	for _, key := range urlKeys(backend) {
		if entry, ok := creds.Accounts[key]; ok && entry.AccessToken != "" {
			return entry.AccessToken
		}
	}
	for _, key := range urlKeys(backend) {
		if token := creds.AccessTokens[key]; token != "" {
			return token
		}
	}
	return ""
}

func urlKeys(backend string) []string {
	trimmed := strings.TrimRight(backend, "/")
	if trimmed == backend {
		return []string{backend, backend + "/"}
	}
	return []string{backend, trimmed}
}

func validateBackend(raw string) *Failure {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" && parsed.Scheme != "file" {
		return &Failure{Category: CategoryUnsupportedBackend}
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		if loopback(parsed.Hostname()) {
			return nil
		}
		return &Failure{Category: CategoryUnsupportedBackend}
	default:
		return &Failure{Category: CategoryUnsupportedBackend}
	}
}

func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func orGetenv(getenv func(string) string) func(string) string {
	if getenv != nil {
		return getenv
	}
	return func(string) string { return "" }
}

func credentialsPath(getenv func(string) string) string {
	if path := getenv("PULUMI_CREDENTIALS_PATH"); path != "" {
		return path
	}
	return filepath.Join(pulumiHome(getenv), "credentials.json")
}

func pulumiHome(getenv func(string) string) string {
	if home := getenv("PULUMI_HOME"); home != "" {
		return home
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".pulumi")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".pulumi")
}

func readCredentials(path string) (*credentialsFile, bool, bool, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, errors.Is(err, os.ErrNotExist), !errors.Is(err, os.ErrNotExist)
	}
	var keys map[string]json.RawMessage
	if err = json.Unmarshal(data, &keys); err != nil {
		return nil, false, false, true
	}
	_, hasCurrent := keys["current"]
	_, hasAccounts := keys["accounts"]
	_, hasTokens := keys["accessTokens"]
	if !hasCurrent && !hasAccounts && !hasTokens {
		return nil, true, false, false
	}
	var parsed credentialsFile
	if err = json.Unmarshal(data, &parsed); err != nil {
		return nil, false, false, true
	}
	return &parsed, false, false, false
}

func readUserConfig(getenv func(string) string) *pulumiUserConfig {
	data, err := os.ReadFile(filepath.Join(pulumiHome(getenv), "config.json"))
	if err != nil {
		return nil
	}
	var parsed pulumiUserConfig
	if err = json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	return &parsed
}
