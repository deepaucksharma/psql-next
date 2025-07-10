// Package config provides configuration loading with secrets management integration
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	
	"github.com/database-intelligence/core/internal/secrets"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/httpprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// SecureConfigLoader loads configuration with integrated secrets management
type SecureConfigLoader struct {
	secretManager *secrets.SecretManager
	logger        *zap.Logger
}

// NewSecureConfigLoader creates a new configuration loader with secrets management
func NewSecureConfigLoader(logger *zap.Logger) *SecureConfigLoader {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &SecureConfigLoader{
		secretManager: secrets.NewSecretManager(logger),
		logger:        logger,
	}
}

// LoadConfig loads configuration from file with secret resolution
func (scl *SecureConfigLoader) LoadConfig(ctx context.Context, configPath string) (*confmap.Conf, error) {
	// Read the configuration file
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse YAML to resolve secrets before passing to confmap
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(content, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// Resolve secrets in the configuration
	if err := scl.resolveSecretsInConfig(ctx, rawConfig); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}
	
	// Convert back to YAML with resolved secrets
	resolvedContent, err := yaml.Marshal(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resolved config: %w", err)
	}
	
	// Create a temporary file with resolved content
	tempFile, err := os.CreateTemp("", "resolved-config-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	
	if _, err := tempFile.Write(resolvedContent); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("failed to write resolved config: %w", err)
	}
	tempFile.Close()
	
	// Use standard confmap providers to load the resolved config
	resolver, err := confmap.NewResolver(confmap.ResolverSettings{
		URIs: []string{"file:" + tempFile.Name()},
		Providers: map[string]confmap.Provider{
			"file": fileprovider.New(),
			"env":  envprovider.New(),
			"yaml": yamlprovider.New(),
			"http": httpprovider.New(),
		},
		Converters: []confmap.Converter{
			expandconverter.New(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create config resolver: %w", err)
	}
	
	conf, err := resolver.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config: %w", err)
	}
	
	scl.logger.Info("Configuration loaded with secrets resolved",
		zap.String("config_path", configPath))
	
	return conf, nil
}

// resolveSecretsInConfig recursively resolves secrets in configuration map
func (scl *SecureConfigLoader) resolveSecretsInConfig(ctx context.Context, config interface{}) error {
	switch v := config.(type) {
	case map[string]interface{}:
		for key, value := range v {
			resolved, err := scl.resolveSecretsInConfig(ctx, value)
			if err != nil {
				return err
			}
			v[key] = resolved
		}
		return nil
		
	case []interface{}:
		for i, value := range v {
			resolved, err := scl.resolveSecretsInConfig(ctx, value)
			if err != nil {
				return err
			}
			v[i] = resolved
		}
		return nil
		
	case string:
		// Check if the string contains secret placeholders
		if strings.Contains(v, "${") && strings.Contains(v, "}") {
			resolved, err := scl.secretManager.ResolvePlaceholders(ctx, v)
			if err != nil {
				return fmt.Errorf("failed to resolve secret in value '%s': %w", v, err)
			}
			return resolved
		}
		return v
		
	default:
		// Return other types as-is
		return config
	}
}

// SecureConfigProvider provides configuration with integrated secrets management
type SecureConfigProvider struct {
	baseProvider  confmap.Provider
	secretManager *secrets.SecretManager
	logger        *zap.Logger
}

// NewSecureConfigProvider wraps a config provider with secrets management
func NewSecureConfigProvider(baseProvider confmap.Provider, logger *zap.Logger) *SecureConfigProvider {
	return &SecureConfigProvider{
		baseProvider:  baseProvider,
		secretManager: secrets.NewSecretManager(logger),
		logger:        logger,
	}
}

// Retrieve gets configuration and resolves secrets
func (scp *SecureConfigProvider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (*confmap.Retrieved, error) {
	// Get the base configuration
	retrieved, err := scp.baseProvider.Retrieve(ctx, uri, watcher)
	if err != nil {
		return nil, err
	}
	
	// Get the raw config
	rawConf, err := retrieved.AsRaw()
	if err != nil {
		return nil, err
	}
	
	// Resolve secrets
	if err := scp.resolveSecretsInConfig(ctx, rawConf); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}
	
	// Return new retrieved with resolved secrets
	return confmap.NewRetrieved(rawConf)
}

// resolveSecretsInConfig is the same as in SecureConfigLoader
func (scp *SecureConfigProvider) resolveSecretsInConfig(ctx context.Context, config interface{}) error {
	switch v := config.(type) {
	case map[string]interface{}:
		for key, value := range v {
			resolved, err := scp.resolveSecretsInConfig(ctx, value)
			if err != nil {
				return err
			}
			v[key] = resolved
		}
		return nil
		
	case []interface{}:
		for i, value := range v {
			resolved, err := scp.resolveSecretsInConfig(ctx, value)
			if err != nil {
				return err
			}
			v[i] = resolved
		}
		return nil
		
	case string:
		if strings.Contains(v, "${") && strings.Contains(v, "}") {
			resolved, err := scp.secretManager.ResolvePlaceholders(ctx, v)
			if err != nil {
				return fmt.Errorf("failed to resolve secret in value '%s': %w", v, err)
			}
			return resolved
		}
		return v
		
	default:
		return config
	}
}

// Scheme returns the scheme of the base provider
func (scp *SecureConfigProvider) Scheme() string {
	return scp.baseProvider.Scheme()
}

// Shutdown shuts down the provider
func (scp *SecureConfigProvider) Shutdown(ctx context.Context) error {
	return scp.baseProvider.Shutdown(ctx)
}

// CreateSecureCollectorSettings creates collector settings with secure config loading
func CreateSecureCollectorSettings(configPaths []string, logger *zap.Logger) (confmap.ResolverSettings, error) {
	// Convert paths to URIs
	uris := make([]string, len(configPaths))
	for i, path := range configPaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return confmap.ResolverSettings{}, fmt.Errorf("failed to get absolute path: %w", err)
		}
		uris[i] = "file:" + absPath
	}
	
	// Create providers with secrets management
	fileProvider := NewSecureConfigProvider(fileprovider.New(), logger)
	envProvider := NewSecureConfigProvider(envprovider.New(), logger)
	yamlProvider := NewSecureConfigProvider(yamlprovider.New(), logger)
	httpProvider := NewSecureConfigProvider(httpprovider.New(), logger)
	
	settings := confmap.ResolverSettings{
		URIs: uris,
		Providers: map[string]confmap.Provider{
			"file": fileProvider,
			"env":  envProvider,
			"yaml": yamlProvider,
			"http": httpProvider,
		},
		Converters: []confmap.Converter{
			expandconverter.New(),
			&secretsConverter{
				secretManager: secrets.NewSecretManager(logger),
				logger:        logger,
			},
		},
	}
	
	return settings, nil
}

// secretsConverter is a custom converter for resolving secrets in config values
type secretsConverter struct {
	secretManager *secrets.SecretManager
	logger        *zap.Logger
}

// Convert resolves secrets in configuration values
func (sc *secretsConverter) Convert(ctx context.Context, conf *confmap.Conf) error {
	// Get all config as a map
	allSettings := conf.ToStringMap()
	
	// Walk through and resolve secrets
	resolved, err := sc.resolveValue(ctx, allSettings)
	if err != nil {
		return err
	}
	
	// Update the conf with resolved values
	if resolvedMap, ok := resolved.(map[string]interface{}); ok {
		for k, v := range resolvedMap {
			if err := conf.Set(k, v); err != nil {
				return fmt.Errorf("failed to set resolved value for %s: %w", k, err)
			}
		}
	}
	
	return nil
}

// resolveValue recursively resolves secret placeholders in values
func (sc *secretsConverter) resolveValue(ctx context.Context, value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Handle special types that should not be resolved
		if configopaque.String(v) == v {
			return v, nil // Don't resolve opaque strings
		}
		
		// Resolve placeholders
		if strings.Contains(v, "${secret:") {
			// Extract secret key
			start := strings.Index(v, "${secret:")
			if start == -1 {
				return v, nil
			}
			
			end := strings.Index(v[start:], "}")
			if end == -1 {
				return nil, fmt.Errorf("malformed secret placeholder")
			}
			
			secretKey := v[start+9 : start+end]
			secretValue, err := sc.secretManager.GetSecret(ctx, secretKey)
			if err != nil {
				// Log warning but don't fail - use original value
				sc.logger.Warn("Failed to resolve secret",
					zap.String("key", secretKey),
					zap.Error(err))
				return v, nil
			}
			
			// Replace the placeholder
			resolved := v[:start] + secretValue + v[start+end+1:]
			
			// Recursively resolve any remaining placeholders
			return sc.resolveValue(ctx, resolved)
		}
		return v, nil
		
	case map[string]interface{}:
		resolvedMap := make(map[string]interface{})
		for k, val := range v {
			resolved, err := sc.resolveValue(ctx, val)
			if err != nil {
				return nil, err
			}
			resolvedMap[k] = resolved
		}
		return resolvedMap, nil
		
	case []interface{}:
		resolvedSlice := make([]interface{}, len(v))
		for i, val := range v {
			resolved, err := sc.resolveValue(ctx, val)
			if err != nil {
				return nil, err
			}
			resolvedSlice[i] = resolved
		}
		return resolvedSlice, nil
		
	default:
		// For other types, check if they're structs we need to handle
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		
		if rv.Kind() == reflect.Struct {
			// Handle special config types
			if _, ok := v.(configtelemetry.Level); ok {
				return v, nil // Don't modify telemetry levels
			}
		}
		
		return v, nil
	}
}