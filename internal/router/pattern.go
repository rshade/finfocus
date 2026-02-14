package router

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/rshade/finfocus/internal/config"
)

// CompiledPattern is a pre-compiled pattern for efficient matching.
type CompiledPattern struct {
	// Original is the original pattern configuration.
	Original config.ResourcePattern

	// Regex is the compiled regex (nil for glob patterns).
	Regex *regexp.Regexp
}

// matchResourceTypeGlob normalizes path separators in a glob pattern and a resource type
// and then applies filepath.Match to determine whether the pattern matches the resource type.
//
// The function replaces "/" characters in both the pattern and resourceType with a
// non-separator sentinel before matching so that glob wildcards do not treat "/" as a
// path separator. This is necessary because Pulumi resource type strings include "/"
// (for example "aws:ec2/instance:Instance") and should be matched as single tokens.
//
// Parameters:
//   - pattern: glob pattern to match against the resource type.
//   - resourceType: resource type string to test against the pattern.
//
// Returns true if the normalized pattern matches the normalized resourceType, and any
// error produced by filepath.Match.
func matchResourceTypeGlob(pattern, resourceType string) (bool, error) {
	// filepath.Match treats path separators specially ("*") doesn't cross them.
	// Pulumi resource types contain "/" (e.g. aws:ec2/instance:Instance), so we
	// normalize "/" to a non-separator sentinel to keep glob behavior intuitive.
	const sepSentinel = "\x00"

	normPattern := strings.ReplaceAll(pattern, "/", sepSentinel)
	normResourceType := strings.ReplaceAll(resourceType, "/", sepSentinel)
	return filepath.Match(normPattern, normResourceType)
}

// Match checks if the pattern matches the given resource type.
func (p *CompiledPattern) Match(resourceType string) (bool, error) {
	if p.Original.IsGlob() {
		return matchResourceTypeGlob(p.Original.Pattern, resourceType)
	}

	if p.Regex != nil {
		return p.Regex.MatchString(resourceType), nil
	}

	return false, fmt.Errorf("pattern not compiled: %s", p.Original.Pattern)
}

// CompilePattern compiles a ResourcePattern for efficient matching.
// It returns a *CompiledPattern containing the original pattern and, if the pattern is a regex,
// the compiled *regexp.Regexp stored in the CompiledPattern.Regex field.
// If the pattern is marked as a regex but fails to compile, it returns an error that includes the
// original pattern and the underlying compilation error.
func CompilePattern(pattern config.ResourcePattern) (*CompiledPattern, error) {
	compiled := &CompiledPattern{
		Original: pattern,
	}

	if pattern.IsRegex() {
		regex, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern.Pattern, err)
		}
		compiled.Regex = regex
	}

	return compiled, nil
}

// PatternCache provides thread-safe caching for compiled regex patterns.
type PatternCache struct {
	mu      sync.RWMutex
	regexes map[string]*regexp.Regexp
}

// NewPatternCache returns a new PatternCache with its internal map for compiled
// NewPatternCache returns a new PatternCache with an initialized, empty regex map ready for concurrent use.
// The returned cache's internal mutex is zero-valued and the map is prepared to store compiled regex patterns.
func NewPatternCache() *PatternCache {
	return &PatternCache{
		regexes: make(map[string]*regexp.Regexp),
	}
}

// MatchGlob matches a glob pattern against a resource type.
func (c *PatternCache) MatchGlob(pattern, resourceType string) (bool, error) {
	return matchResourceTypeGlob(pattern, resourceType)
}

// MatchRegex matches a regex pattern against a resource type.
// Compiled regexes are cached for performance.
func (c *PatternCache) MatchRegex(pattern, resourceType string) (bool, error) {
	c.mu.RLock()
	re, ok := c.regexes[pattern]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		// Double-check after acquiring write lock
		re, ok = c.regexes[pattern]
		if !ok {
			var err error
			re, err = regexp.Compile(pattern)
			if err != nil {
				c.mu.Unlock()
				return false, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
			}
			c.regexes[pattern] = re
		}
		c.mu.Unlock()
	}

	return re.MatchString(resourceType), nil
}

// Match matches a pattern against a resource type based on the pattern type.
func (c *PatternCache) Match(pattern config.ResourcePattern, resourceType string) (bool, error) {
	if pattern.IsGlob() {
		return c.MatchGlob(pattern.Pattern, resourceType)
	}
	return c.MatchRegex(pattern.Pattern, resourceType)
}

// Clear clears the pattern cache.
func (c *PatternCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.regexes = make(map[string]*regexp.Regexp)
}

// Size returns the number of cached regex patterns.
func (c *PatternCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.regexes)
}