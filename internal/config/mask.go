package config

import "maps"

// RedactedValue replaces a secret notification value in displayed configuration.
const RedactedValue = "[REDACTED]"

// MaskedForDisplay returns a copy of c whose notification destination URLs and
// header values are replaced by RedactedValue, except a value that is exactly
// one ${NAME} reference, which holds no secret and is shown as written.
//
// Use it only for display (config get, config list). Config.Save marshals the
// struct, so masking must never touch the configuration that is saved.
func (c *Config) MaskedForDisplay() *Config {
	masked := *c
	masked.Cost.Budgets = cloneBudgets(c.Cost.Budgets)
	forEachDestination(masked.Cost.Budgets, func(_ string, dest *NotificationDestination) {
		dest.URL = maskNotificationValue(dest.URL)
		if dest.Headers == nil {
			return
		}
		headers := make(map[string]string, len(dest.Headers))
		for name, value := range dest.Headers {
			headers[name] = maskNotificationValue(value)
		}
		dest.Headers = headers
	})
	masked.Plugins = maskPlugins(c.Plugins)
	return &masked
}

// maskPlugins copies plugin settings and replaces a non-empty credential value
// with RedactedValue. An empty credential stays empty so a missing secret is
// distinguishable from one that is set.
func maskPlugins(plugins map[string]PluginConfig) map[string]PluginConfig {
	if plugins == nil {
		return nil
	}
	masked := make(map[string]PluginConfig, len(plugins))
	for name, plugin := range plugins {
		cloned := make(map[string]any, len(plugin.Config))
		for key, value := range plugin.Config {
			if credentialSetting(name, key) && presentSecret(value) {
				cloned[key] = RedactedValue
				continue
			}
			cloned[key] = maskNestedCredentials(value)
		}
		masked[name] = PluginConfig{Config: cloned}
	}
	return masked
}

func maskNestedCredentials(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		masked := make(map[string]any, len(typed))
		for key, child := range typed {
			if IsCredentialName(key) && presentSecret(child) {
				masked[key] = RedactedValue
				continue
			}
			masked[key] = maskNestedCredentials(child)
		}
		return masked
	case []any:
		masked := make([]any, len(typed))
		for i, child := range typed {
			masked[i] = maskNestedCredentials(child)
		}
		return masked
	default:
		return value
	}
}

func credentialSetting(pluginName, key string) bool {
	return IsCredentialName(key) || IsCredentialName(pluginName)
}

func presentSecret(value any) bool {
	if value == nil {
		return false
	}
	text, ok := value.(string)
	if ok {
		return text != ""
	}
	return true
}

func maskNotificationValue(value string) string {
	if value == "" {
		return ""
	}
	if IsSingleNotificationReference(value) {
		return value
	}
	return RedactedValue
}

// cloneBudgets deep-copies the scopes and alert lists of budgets so masking a
// destination cannot change the original configuration.
func cloneBudgets(budgets *BudgetsConfig) *BudgetsConfig {
	if budgets == nil {
		return nil
	}
	cloned := *budgets
	cloned.Global = cloneScope(budgets.Global)
	if budgets.Providers != nil {
		cloned.Providers = make(map[string]*ScopedBudget, len(budgets.Providers))
		for name, scope := range budgets.Providers {
			cloned.Providers[name] = cloneScope(scope)
		}
	}
	if budgets.Tags != nil {
		cloned.Tags = make([]TagBudget, len(budgets.Tags))
		for i, tag := range budgets.Tags {
			tag.ScopedBudget = *cloneScope(&tag.ScopedBudget)
			cloned.Tags[i] = tag
		}
	}
	if budgets.Types != nil {
		cloned.Types = make(map[string]*ScopedBudget, len(budgets.Types))
		for name, scope := range budgets.Types {
			cloned.Types[name] = cloneScope(scope)
		}
	}
	return &cloned
}

func cloneScope(scope *ScopedBudget) *ScopedBudget {
	if scope == nil {
		return nil
	}
	cloned := *scope
	if scope.Alerts != nil {
		cloned.Alerts = make([]AlertConfig, len(scope.Alerts))
		for i, alert := range scope.Alerts {
			if alert.Notifications != nil {
				dests := make([]NotificationDestination, len(alert.Notifications))
				for j, dest := range alert.Notifications {
					dest.Headers = maps.Clone(dest.Headers)
					dests[j] = dest
				}
				alert.Notifications = dests
			}
			cloned.Alerts[i] = alert
		}
	}
	return &cloned
}
