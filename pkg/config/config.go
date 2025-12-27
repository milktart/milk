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
