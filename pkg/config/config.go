package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dlclark/regexp2"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed patterns.yaml
	patternsYAMLBytes []byte

	//go:embed regions.yaml
	regionsYAMLBytes []byte
)

// Note: This package loads configuration from YAML files in the config/ directory

// Config holds all configuration data
type Config struct {
	Patterns PatternsConfig `yaml:"patterns"`
	Regions  RegionsConfig  `yaml:"regions"`

	// Compiled regexes (not in YAML)
	CompiledVIP      []*regexp2.Regexp
	CompiledPlatinum []*regexp2.Regexp
	CompiledNotable  []*regexp2.Regexp
}

// PatternsConfig holds regex patterns organized by tier
type PatternsConfig struct {
	VIP      []string `yaml:"vip"`
	Platinum []string `yaml:"platinum"`
	Notable  []string `yaml:"notable"`
}

// RegionsConfig holds region code mappings
type RegionsConfig map[string][]string

var cfg *Config

// UserPatternsFile returns the path to the user's local patterns override file.
func UserPatternsFile() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "milk", "numbers", "patterns.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "milk", "numbers", "patterns.yaml")
}

// EmbeddedPatternsYAML returns the raw embedded patterns YAML bytes.
func EmbeddedPatternsYAML() []byte {
	return patternsYAMLBytes
}

// LoadWithUserOverride loads config using the user's local patterns file if it
// exists, otherwise falling back to the embedded patterns.
func LoadWithUserOverride() (*Config, error) {
	patternsData := patternsYAMLBytes
	if data, err := os.ReadFile(UserPatternsFile()); err == nil {
		patternsData = data
	}

	var patternsYAML struct {
		Patterns PatternsConfig `yaml:"patterns"`
	}
	if err := yaml.Unmarshal(patternsData, &patternsYAML); err != nil {
		return nil, fmt.Errorf("failed to parse patterns.yaml: %w", err)
	}

	var regionsYAML struct {
		Regions RegionsConfig `yaml:"regions"`
	}
	if err := yaml.Unmarshal(regionsYAMLBytes, &regionsYAML); err != nil {
		return nil, fmt.Errorf("failed to parse regions.yaml: %w", err)
	}

	cfg = &Config{
		Patterns: patternsYAML.Patterns,
		Regions:  regionsYAML.Regions,
	}

	if err := compileRegexes(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Load loads and parses configuration from YAML files
func Load(configDir string) (*Config, error) {
	// Load patterns
	patternsFile := filepath.Join(configDir, "patterns.yaml")
	patternsData, err := os.ReadFile(patternsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read patterns.yaml: %w", err)
	}

	var patternsYAML struct {
		Patterns PatternsConfig `yaml:"patterns"`
	}

	if err := yaml.Unmarshal(patternsData, &patternsYAML); err != nil {
		return nil, fmt.Errorf("failed to parse patterns.yaml: %w", err)
	}

	// Load regions
	regionsFile := filepath.Join(configDir, "regions.yaml")
	regionsData, err := os.ReadFile(regionsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read regions.yaml: %w", err)
	}

	var regionsYAML struct {
		Regions RegionsConfig `yaml:"regions"`
	}

	if err := yaml.Unmarshal(regionsData, &regionsYAML); err != nil {
		return nil, fmt.Errorf("failed to parse regions.yaml: %w", err)
	}

	cfg = &Config{
		Patterns: patternsYAML.Patterns,
		Regions:  regionsYAML.Regions,
	}

	// Compile regexes
	if err := compileRegexes(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromBytes loads and parses configuration from embedded YAML bytes
func LoadFromBytes() (*Config, error) {
	var patternsYAML struct {
		Patterns PatternsConfig `yaml:"patterns"`
	}

	if err := yaml.Unmarshal(patternsYAMLBytes, &patternsYAML); err != nil {
		return nil, fmt.Errorf("failed to parse patterns.yaml: %w", err)
	}

	var regionsYAML struct {
		Regions RegionsConfig `yaml:"regions"`
	}

	if err := yaml.Unmarshal(regionsYAMLBytes, &regionsYAML); err != nil {
		return nil, fmt.Errorf("failed to parse regions.yaml: %w", err)
	}

	cfg = &Config{
		Patterns: patternsYAML.Patterns,
		Regions:  regionsYAML.Regions,
	}

	// Compile regexes
	if err := compileRegexes(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// compileRegexes compiles all regex patterns
func compileRegexes() error {
	for _, p := range cfg.Patterns.VIP {
		re, err := regexp2.Compile(p, 0)
		if err != nil {
			return fmt.Errorf("failed to compile VIP pattern '%s': %w", p, err)
		}
		cfg.CompiledVIP = append(cfg.CompiledVIP, re)
	}

	for _, p := range cfg.Patterns.Platinum {
		re, err := regexp2.Compile(p, 0)
		if err != nil {
			return fmt.Errorf("failed to compile Platinum pattern '%s': %w", p, err)
		}
		cfg.CompiledPlatinum = append(cfg.CompiledPlatinum, re)
	}

	for _, p := range cfg.Patterns.Notable {
		re, err := regexp2.Compile(p, 0)
		if err != nil {
			return fmt.Errorf("failed to compile Notable pattern '%s': %w", p, err)
		}
		cfg.CompiledNotable = append(cfg.CompiledNotable, re)
	}

	return nil
}

// Get returns the current config instance
func Get() *Config {
	return cfg
}

// GetRegionCodes retrieves area codes for a region
func (c *Config) GetRegionCodes(region string) []string {
	if codes, ok := c.Regions[region]; ok {
		return codes
	}
	return nil
}

// saveUserPatterns writes the current patterns to the user's local patterns file.
func saveUserPatterns(p PatternsConfig) error {
	path := UserPatternsFile()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	out := struct {
		Patterns PatternsConfig `yaml:"patterns"`
	}{Patterns: p}
	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("could not marshal patterns: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// loadUserPatterns returns the patterns from the user's local file, seeding it
// from the embedded default first if it doesn't exist yet.
func loadUserPatterns() (PatternsConfig, error) {
	path := UserPatternsFile()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return PatternsConfig{}, err
		}
		if err := os.WriteFile(path, patternsYAMLBytes, 0644); err != nil {
			return PatternsConfig{}, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PatternsConfig{}, err
	}
	var wrapper struct {
		Patterns PatternsConfig `yaml:"patterns"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return PatternsConfig{}, fmt.Errorf("could not parse user patterns: %w", err)
	}
	return wrapper.Patterns, nil
}

// AddPattern appends a pattern string to the given tier in the user's local
// patterns file. tier must be "vip", "platinum", or "notable".
func AddPattern(tier, pattern string) error {
	p, err := loadUserPatterns()
	if err != nil {
		return err
	}
	switch tier {
	case "vip":
		p.VIP = append(p.VIP, pattern)
	case "platinum":
		p.Platinum = append(p.Platinum, pattern)
	case "notable":
		p.Notable = append(p.Notable, pattern)
	default:
		return fmt.Errorf("unknown tier %q (use vip, platinum, or notable)", tier)
	}
	if err := saveUserPatterns(p); err != nil {
		return err
	}
	fmt.Printf("Added pattern to %s: %s\n", tier, pattern)
	return nil
}

// PatternEntry is a flat representation of a pattern with its tier, used for
// interactive selection.
type PatternEntry struct {
	Tier    string
	Pattern string
}

// ListPatterns returns all patterns from the user's local file (seeding from
// embedded if needed), as a flat slice with tier information.
func ListPatterns() ([]PatternEntry, error) {
	p, err := loadUserPatterns()
	if err != nil {
		return nil, err
	}
	var entries []PatternEntry
	for _, s := range p.VIP {
		entries = append(entries, PatternEntry{Tier: "vip", Pattern: s})
	}
	for _, s := range p.Platinum {
		entries = append(entries, PatternEntry{Tier: "platinum", Pattern: s})
	}
	for _, s := range p.Notable {
		entries = append(entries, PatternEntry{Tier: "notable", Pattern: s})
	}
	return entries, nil
}

// RemovePatterns removes the patterns at the given flat indices (as returned by
// ListPatterns) from the user's local patterns file.
func RemovePatterns(indices []int, entries []PatternEntry) error {
	remove := make(map[int]bool, len(indices))
	for _, i := range indices {
		remove[i] = true
	}

	p, err := loadUserPatterns()
	if err != nil {
		return err
	}

	// Rebuild each tier by walking entries and skipping removed ones.
	vipIdx, platIdx, notableIdx := 0, 0, 0
	var newVIP, newPlat, newNotable []string
	for i, e := range entries {
		switch e.Tier {
		case "vip":
			if !remove[i] {
				newVIP = append(newVIP, p.VIP[vipIdx])
			}
			vipIdx++
		case "platinum":
			if !remove[i] {
				newPlat = append(newPlat, p.Platinum[platIdx])
			}
			platIdx++
		case "notable":
			if !remove[i] {
				newNotable = append(newNotable, p.Notable[notableIdx])
			}
			notableIdx++
		}
	}
	p.VIP = newVIP
	p.Platinum = newPlat
	p.Notable = newNotable

	return saveUserPatterns(p)
}
