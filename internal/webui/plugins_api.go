package webui

// StatusError is a callback failure the plugins handlers map to HTTP.
// Text is safe to show: no secret, token, or URL.
type StatusError struct {
	Status int
	Code   string
	Text   string
}

// Error returns the safe message.
func (e *StatusError) Error() string {
	if e == nil {
		return ""
	}
	return e.Text
}

// PluginCatalogRow is one Plugins page row.
type PluginCatalogRow struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Providers         []string `json:"providers"`
	Capabilities      []string `json:"capabilities"`
	SecurityLevel     string   `json:"securityLevel"`
	Repository        string   `json:"repository"`
	InCatalog         bool     `json:"inCatalog"`
	InstalledVersions []string `json:"installedVersions"`
	LatestVersion     string   `json:"latestVersion"`
	UpdateAvailable   bool     `json:"updateAvailable"`
	Warning           string   `json:"warning,omitempty"`
	Deprecated        bool     `json:"deprecated,omitempty"`
	Replacement       string   `json:"replacement,omitempty"`
}

// PluginPlanRequest asks for a confirmation summary. Version is optional for
// install and update. Remove requires it.
type PluginPlanRequest struct {
	Action  string `json:"action"`
	Version string `json:"version,omitempty"`
}

// PluginPlan is the confirmation summary. Building it does not download or delete.
type PluginPlan struct {
	Action        string `json:"action"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Repository    string `json:"repository"`
	SecurityLevel string `json:"securityLevel"`
	FallbackFrom  string `json:"fallbackFrom"`
	ChecksumFile  string `json:"checksumFile"`
	InCatalog     bool   `json:"inCatalog"`
	Deprecated    bool   `json:"deprecated,omitempty"`
	Replacement   string `json:"replacement,omitempty"`
}

// PluginOperationRequest confirms a plan. CatalogOnly and RequireChecksumFile
// are set by the handler before the callback runs; they are not part of the body.
type PluginOperationRequest struct {
	Version               string `json:"version"`
	Repository            string `json:"repository"`
	SecurityLevel         string `json:"securityLevel"`
	AcceptMissingChecksum bool   `json:"acceptMissingChecksum"`
	AcceptFallbackFrom    string `json:"acceptFallbackFrom"`
	CatalogOnly           bool   `json:"-"`
	RequireChecksumFile   bool   `json:"-"`
}

// PluginProgress is one install, update, or remove step.
type PluginProgress struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
	Bytes   int64  `json:"bytes"`
	Total   int64  `json:"total"`
	Warning bool   `json:"warning"`
}

// PluginAccepted is the 202 body for a started operation.
type PluginAccepted struct {
	OperationID string `json:"operationId"`
}

// PluginOperationSnapshot is the first SSE event for an operation.
type PluginOperationSnapshot struct {
	Status string           `json:"status"`
	Events []PluginProgress `json:"events"`
	Error  string           `json:"error"`
}

// PluginOperationDone is the terminal SSE event.
type PluginOperationDone struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// PluginValidateResult is the outcome of the file check and GetPluginInfo.
type PluginValidateResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

// PluginSettingDescriptor is one advertised setting. A secret default is
// reported through DefaultSet only.
type PluginSettingDescriptor struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Secret      bool     `json:"secret"`
	Default     string   `json:"default,omitempty"`
	DefaultSet  bool     `json:"defaultSet"`
	Description string   `json:"description"`
	Env         string   `json:"env"`
	EnumValues  []string `json:"enumValues,omitempty"`
}

// PluginValueSide is one tier of a setting. Secret rows leave Value empty.
type PluginValueSide struct {
	Set   bool   `json:"set"`
	Value string `json:"value,omitempty"`
}

// PluginSettingValue is the stored state of one key.
type PluginSettingValue struct {
	Key             string          `json:"key"`
	Secret          bool            `json:"secret"`
	Project         PluginValueSide `json:"project"`
	Global          PluginValueSide `json:"global"`
	EnvSet          bool            `json:"envSet"`
	EffectiveSource string          `json:"effectiveSource"`
}

// PluginSettingsView is the settings form for one plugin.
type PluginSettingsView struct {
	Name             string                    `json:"name"`
	Mode             string                    `json:"mode"`
	Descriptors      []PluginSettingDescriptor `json:"descriptors"`
	Versions         map[string]string         `json:"versions"`
	ProjectAvailable bool                      `json:"projectAvailable"`
	Values           []PluginSettingValue      `json:"values"`
}

// PluginRun reports progress and returns when the install, update, or remove finishes.
type PluginRun func(PluginProgress)

// pluginSettingsSaved is the save message for the plugins page.
const pluginSettingsSaved = "Saved. These settings apply the next time this plugin is launched."

// stripSecretSettings removes every secret value before the response is written.
func stripSecretSettings(view PluginSettingsView) PluginSettingsView {
	for i := range view.Descriptors {
		if view.Descriptors[i].Secret {
			view.Descriptors[i].Default = ""
		}
	}
	for i := range view.Values {
		if !view.Values[i].Secret {
			continue
		}
		view.Values[i].Project.Value = ""
		view.Values[i].Global.Value = ""
	}
	if view.Descriptors == nil {
		view.Descriptors = []PluginSettingDescriptor{}
	}
	if view.Values == nil {
		view.Values = []PluginSettingValue{}
	}
	return view
}

// pluginChangeKey reports whether key is plugins.<name>.<field> for this plugin.
func pluginChangeKey(name, key string) bool {
	prefix := "plugins." + name + "."
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return false
	}
	rest := key[len(prefix):]
	return rest != "" && !containsDot(rest)
}

func containsDot(value string) bool {
	for _, r := range value {
		if r == '.' {
			return true
		}
	}
	return false
}

// confirmationMatches reports whether body still agrees with a fresh plan.
func confirmationMatches(plan PluginPlan, body PluginOperationRequest) bool {
	if body.Version != plan.Version || body.Repository != plan.Repository || body.SecurityLevel != plan.SecurityLevel {
		return false
	}
	if body.AcceptFallbackFrom != plan.FallbackFrom {
		return false
	}
	if plan.ChecksumFile == "missing" && !body.AcceptMissingChecksum {
		return false
	}
	return true
}

// pluginSlug reports whether name is a catalog name rather than a URL or owner/repo.
func pluginSlug(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}
