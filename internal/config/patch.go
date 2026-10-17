package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"
)

const (
	// AbsentVersion is the version of a configuration file that does not exist.
	AbsentVersion = "absent"
	// TierGlobal and TierProject select which validation rules a write uses.
	TierGlobal  = "global"
	TierProject = "project"
)

var (
	// ErrStaleConfig is returned when a write's base version does not match the file.
	ErrStaleConfig = errors.New("configuration changed since it was loaded; reload and try again")
	// ErrProjectUnavailable is returned when a project write has no project directory.
	ErrProjectUnavailable = errors.New("no project directory is available")
)

// WriteRequest is a preview or save submitted by the dashboard.
type WriteRequest struct {
	Tier        string   `json:"tier"`
	BaseVersion string   `json:"baseVersion"`
	Changes     []Change `json:"changes"`
}

// Change is one edited setting. A nil Value with Clear false leaves a secret
// unchanged. The masked placeholder is never written.
type Change struct {
	Key   string  `json:"key"`
	Value *string `json:"value,omitempty"`
	Clear bool    `json:"clear,omitempty"`
}

// DiffEntry is one preview line. Secret values are status words, never the stored text.
type DiffEntry struct {
	Key    string `json:"key"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// PatchResult is a dry-run or a completed write.
type PatchResult struct {
	Version    string
	Diff       []DiffEntry
	Validation ValidationResult
}

// FileVersion returns the SHA-256 hex of path, or AbsentVersion when it is missing.
func FileVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return AbsentVersion, nil
		}
		return "", fmt.Errorf("reading configuration: %w", err)
	}
	return hashBytes(data), nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TierPath resolves the file a tier write updates. An empty project directory
// cannot select the project tier.
func TierPath(globalPath, projectDir, tier string) (string, error) {
	switch tier {
	case TierGlobal:
		if globalPath == "" {
			return "", errors.New("configuration path is unavailable")
		}
		return globalPath, nil
	case TierProject:
		if projectDir == "" {
			return "", ErrProjectUnavailable
		}
		return ProjectConfigPath(projectDir), nil
	default:
		return "", invalidChange("unknown configuration tier %q", tier)
	}
}

// PatchFile applies changes to the tier file at path. DryRun writes nothing.
// baseVersion is the caller's observed FileVersion. An empty baseVersion skips
// the conflict check. path is the resolved tier file; it is never taken from
// the changes.
func PatchFile(path, tier, baseVersion string, changes []Change, dryRun bool) (PatchResult, error) {
	if path == "" {
		return PatchResult{}, ErrProjectUnavailable
	}
	target := hujsonPath(path)
	unlock, err := acquirePathLock(target)
	if err != nil {
		return PatchResult{}, err
	}
	defer unlock()

	current, version, err := readTier(target)
	if err != nil {
		return PatchResult{}, err
	}
	if baseVersion != "" && baseVersion != version {
		return PatchResult{}, ErrStaleConfig
	}

	next, diff, err := applyChanges(current, changes)
	if err != nil {
		return PatchResult{}, err
	}
	validation := validateTier(target, tier, next)
	result := PatchResult{Version: version, Diff: diff, Validation: validation}
	if !validation.Valid {
		return result, &TierValidationError{Result: validation}
	}
	if dryRun || len(diff) == 0 {
		return result, nil
	}
	written, err := writeAtomic(target, next)
	if err != nil {
		return PatchResult{}, err
	}
	result.Version = hashBytes(written)
	return result, nil
}

// InvalidChangeError is a change the caller got wrong, such as an unknown key
// or a write to a nested value. I/O failures are not InvalidChangeError.
type InvalidChangeError struct {
	Message string
}

func (e *InvalidChangeError) Error() string { return e.Message }

func invalidChange(format string, args ...any) error {
	return &InvalidChangeError{Message: fmt.Sprintf(format, args...)}
}

// TierValidationError is a rejected write. The target file is unchanged.
type TierValidationError struct {
	Result ValidationResult
}

func (e *TierValidationError) Error() string {
	if e == nil || len(e.Result.Errors) == 0 {
		return "configuration is invalid"
	}
	return e.Result.Errors[0].Message
}

func hujsonPath(path string) string {
	if isYAMLPath(path) {
		return filepath.Join(filepath.Dir(path), "config.hujson")
	}
	return path
}

// PreferredConfigPath is the file a reader should hash: the HuJSON file when
// it exists, otherwise a legacy YAML sibling, otherwise the HuJSON path.
func PreferredConfigPath(path string) string {
	target := hujsonPath(path)
	if _, err := os.Stat(target); err == nil {
		return target
	}
	legacy := filepath.Join(filepath.Dir(target), "config.yaml")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return target
}

// readTier returns the bytes to patch and the version FileVersion reports for
// the preferred path. A legacy YAML file is returned as-is so its hash matches
// the view; applyChanges converts it before the patch.
func readTier(target string) ([]byte, string, error) {
	preferred := PreferredConfigPath(target)
	data, err := os.ReadFile(preferred)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []byte("{}\n"), AbsentVersion, nil
		}
		return nil, "", fmt.Errorf("reading configuration: %w", err)
	}
	return data, hashBytes(data), nil
}

func validateTier(path, tier string, data []byte) ValidationResult {
	var result ValidationResult
	if tier == TierProject {
		result, _ = ValidateProjectConfigSource(path, data)
		return result
	}
	result, _ = ValidateConfigSource(path, data)
	return result
}

func applyChanges(original []byte, changes []Change) ([]byte, []DiffEntry, error) {
	if len(bytes.TrimSpace(original)) == 0 {
		original = []byte("{}\n")
	}
	if isYAMLDocument(original) {
		converted, err := yamlToJSON(original)
		if err != nil {
			return nil, nil, fmt.Errorf("converting legacy configuration: %w", err)
		}
		original = converted
	}
	value, err := hujson.Parse(original)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing configuration: %w", err)
	}
	var diff []DiffEntry
	for _, change := range changes {
		entry, skip, changeErr := applyChange(&value, change)
		if changeErr != nil {
			return nil, nil, changeErr
		}
		if skip {
			continue
		}
		diff = append(diff, entry)
	}
	if diff == nil {
		diff = []DiffEntry{}
	}
	return value.Pack(), diff, nil
}

func isYAMLDocument(data []byte) bool {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 || trim[0] == '{' || trim[0] == '[' {
		return false
	}
	// HuJSON documents may start with a comment. YAML comments start with '#'.
	return !bytes.HasPrefix(trim, []byte("//")) && !bytes.HasPrefix(trim, []byte("/*"))
}

func applyChange(value *hujson.Value, change Change) (DiffEntry, bool, error) {
	if isAlertsKey(change.Key) {
		return DiffEntry{}, false, invalidChange("cost.budgets.alerts must be configured via YAML")
	}
	key := CanonicalKey(change.Key)
	setting, ok := LookupSetting(key)
	if !ok {
		return DiffEntry{}, false, invalidChange("unknown configuration section: %s", sectionName(key))
	}
	pointer, err := pointerFor(key)
	if err != nil {
		return DiffEntry{}, false, err
	}
	return applyKnownChange(value, setting, key, pointer, change)
}

func isAlertsKey(key string) bool {
	return key == "cost.budgets.alerts" || strings.HasPrefix(key, "cost.budgets.alerts.")
}

func applyKnownChange(
	value *hujson.Value,
	setting Setting,
	key, pointer string,
	change Change,
) (DiffEntry, bool, error) {
	secret := setting.Secret || IsSecretSetting(key)
	if !change.Clear && unchangedSecret(change.Value, secret) {
		return DiffEntry{}, true, nil
	}
	before := readAt(value, pointer)
	beforeText := formatStored(before, secret)
	if shouldClearSetting(setting, change) {
		return clearSetting(value, pointer, key, before, beforeText)
	}
	if change.Value == nil {
		return DiffEntry{}, true, nil
	}
	setting, err := storedPluginSetting(setting, key, before)
	if err != nil {
		return DiffEntry{}, false, err
	}
	return writeSetting(value, setting, key, pointer, *change.Value, secret, beforeText)
}

// storedPluginSetting keeps a plugin value's stored type. A plugin may read
// numbers and booleans, and a nested object or array holds data the page cannot
// show, so neither is replaced by text.
func storedPluginSetting(setting Setting, key string, before *hujson.Value) (Setting, error) {
	if setting.Kind != KindPlugin || before == nil {
		return setting, nil
	}
	switch kind := before.Value.Kind(); {
	case kind == '{' || kind == '[':
		return setting, invalidChange("%s holds a nested value; edit the file directly", key)
	case kind == 't' || kind == 'f':
		setting.Kind = KindBool
	case kind == '-' || kind >= '0' && kind <= '9':
		setting.Kind = KindNumber
	}
	return setting, nil
}

func shouldClearSetting(setting Setting, change Change) bool {
	if change.Clear {
		return true
	}
	return setting.Kind == KindClusterContext && change.Value != nil && *change.Value == ""
}

func clearSetting(
	value *hujson.Value,
	pointer, key string,
	before *hujson.Value,
	beforeText string,
) (DiffEntry, bool, error) {
	if before == nil {
		return DiffEntry{}, true, nil
	}
	if err := removeAtPointer(value, pointer); err != nil {
		return DiffEntry{}, false, err
	}
	return DiffEntry{Key: key, Before: beforeText, After: "not set"}, false, nil
}

func writeSetting(
	value *hujson.Value,
	setting Setting,
	key, pointer, raw string,
	secret bool,
	beforeText string,
) (DiffEntry, bool, error) {
	if secret && strings.TrimSpace(raw) == "" {
		return DiffEntry{}, false, invalidChange("%s must not be empty", key)
	}
	encoded, err := encodeSetting(setting, raw)
	if err != nil {
		return DiffEntry{}, false, &TierValidationError{Result: ValidationResult{
			Errors: []ValidationError{{Path: key, Message: err.Error()}},
		}}
	}
	literal, err := json.Marshal(encoded)
	if err != nil {
		return DiffEntry{}, false, err
	}
	afterText := string(literal)
	if secret {
		afterText = "replaced"
	}
	if !secret && beforeText == afterText {
		return DiffEntry{}, true, nil
	}
	if err = setAtPointer(value, pointer, literal); err != nil {
		return DiffEntry{}, false, err
	}
	return DiffEntry{Key: key, Before: beforeText, After: afterText}, false, nil
}

func unchangedSecret(value *string, secret bool) bool {
	if !secret || value == nil {
		return false
	}
	return *value == RedactedValue
}

func formatStored(value *hujson.Value, secret bool) string {
	if value == nil {
		if secret {
			return "not set"
		}
		return ""
	}
	if secret {
		return "set"
	}
	cloned := value.Clone()
	cloned.Standardize()
	var decoded any
	if err := json.Unmarshal(cloned.Pack(), &decoded); err != nil {
		return ""
	}
	packed, err := json.Marshal(redactNestedSecrets(decoded))
	if err != nil {
		return ""
	}
	return string(packed)
}

func readAt(value *hujson.Value, pointer string) *hujson.Value {
	return value.Find(pointer)
}

func removeAtPointer(value *hujson.Value, pointer string) error {
	patch := fmt.Sprintf(`[{"op":"remove","path":%q}]`, pointer)
	if err := value.Patch([]byte(patch)); err != nil {
		return fmt.Errorf("removing %s: %w", pointer, err)
	}
	return nil
}

func setAtPointer(value *hujson.Value, pointer string, literal []byte) error {
	if err := ensureParents(value, pointer); err != nil {
		return err
	}
	op := "add"
	if value.Find(pointer) != nil {
		op = "replace"
	}
	patch := fmt.Sprintf(`[{"op":%q,"path":%q,"value":%s}]`, op, pointer, literal)
	if err := value.Patch([]byte(patch)); err != nil {
		return fmt.Errorf("setting %s: %w", pointer, err)
	}
	return nil
}

func ensureParents(value *hujson.Value, pointer string) error {
	segments := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	const minPointerSegments = 2
	if len(segments) < minPointerSegments {
		return nil
	}
	var built strings.Builder
	for _, segment := range segments[:len(segments)-1] {
		built.WriteByte('/')
		built.WriteString(segment)
		parent := built.String()
		if value.Find(parent) != nil {
			continue
		}
		patch := fmt.Sprintf(`[{"op":"add","path":%q,"value":{}}]`, parent)
		if err := value.Patch([]byte(patch)); err != nil {
			return fmt.Errorf("creating %s: %w", parent, err)
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) ([]byte, error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(append([]byte{}, data...), '\n')
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("creating config temp file: %w", err)
	}
	name := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("writing config: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("writing config: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return nil, fmt.Errorf("writing config: %w", err)
	}
	if err = os.Rename(name, path); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}
	cleanup = false
	return data, nil
}
