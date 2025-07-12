package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SecretManager provides secure secret management with multiple providers
type SecretManager struct {
	providers map[string]Provider
	cache     map[string]*cachedSecret
	logger    *zap.Logger
	mu        sync.RWMutex
}

// Provider defines the interface for secret providers
type Provider interface {
	GetSecret(ctx context.Context, key string) (string, error)
	Type() string
	IsAvailable() bool
}

// cachedSecret represents a cached secret with expiration
type cachedSecret struct {
	value     string
	expiresAt time.Time
}

// NewSecretManager creates a new secret manager with multiple providers
func NewSecretManager(logger *zap.Logger) *SecretManager {
	// Use a no-op logger if none provided to prevent nil pointer dereference
	if logger == nil {
		logger = zap.NewNop()
	}
	
	sm := &SecretManager{
		providers: make(map[string]Provider),
		cache:     make(map[string]*cachedSecret),
		logger:    logger,
	}

	// Initialize providers in order of preference
	sm.registerProvider(&EnvProvider{})
	sm.registerProvider(&KubernetesProvider{})
	sm.registerProvider(&VaultProvider{})

	return sm
}

// registerProvider adds a provider to the manager
func (sm *SecretManager) registerProvider(provider Provider) {
	if provider.IsAvailable() {
		sm.providers[provider.Type()] = provider
		sm.logger.Info("Registered secret provider", zap.String("type", provider.Type()))
	} else {
		sm.logger.Debug("Secret provider not available", zap.String("type", provider.Type()))
	}
}

// GetSecret retrieves a secret using the best available provider
func (sm *SecretManager) GetSecret(ctx context.Context, key string) (string, error) {
	// Check cache first
	sm.mu.RLock()
	if cached, exists := sm.cache[key]; exists && time.Now().Before(cached.expiresAt) {
		sm.mu.RUnlock()
		return cached.value, nil
	}
	sm.mu.RUnlock()

	// Try providers in order of preference
	providerOrder := []string{"vault", "kubernetes", "env"}
	
	for _, providerType := range providerOrder {
		if provider, exists := sm.providers[providerType]; exists {
			value, err := provider.GetSecret(ctx, key)
			if err == nil && value != "" {
				// Cache the secret for 5 minutes
				sm.cacheSecret(key, value, 5*time.Minute)
				sm.logger.Debug("Retrieved secret from provider", 
					zap.String("key", key),
					zap.String("provider", providerType))
				return value, nil
			}
			sm.logger.Debug("Provider failed to retrieve secret", 
				zap.String("key", key),
				zap.String("provider", providerType),
				zap.Error(err))
		}
	}

	return "", fmt.Errorf("failed to retrieve secret '%s' from any provider", key)
}

// cacheSecret stores a secret in cache with expiration
func (sm *SecretManager) cacheSecret(key, value string, ttl time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.cache[key] = &cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// ClearCache removes all cached secrets
func (sm *SecretManager) ClearCache() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.cache = make(map[string]*cachedSecret)
	sm.logger.Info("Cleared secret cache")
}

// EnvProvider gets secrets from environment variables
type EnvProvider struct{}

func (p *EnvProvider) GetSecret(ctx context.Context, key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s not found", key)
	}
	return value, nil
}

func (p *EnvProvider) Type() string {
	return "env"
}

func (p *EnvProvider) IsAvailable() bool {
	return true // Always available
}

// KubernetesProvider gets secrets from Kubernetes secrets
type KubernetesProvider struct{}

func (p *KubernetesProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Check if we're running in Kubernetes
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in Kubernetes")
	}

	// Map secret key to file path
	secretPath := "/etc/secrets/" + strings.ReplaceAll(key, "_", "-")
	
	content, err := os.ReadFile(secretPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Kubernetes secret: %w", err)
	}
	
	return strings.TrimSpace(string(content)), nil
}

func (p *KubernetesProvider) Type() string {
	return "kubernetes"
}

func (p *KubernetesProvider) IsAvailable() bool {
	// Check if we're running in Kubernetes
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount")
	return err == nil
}

// VaultProvider gets secrets from HashiCorp Vault
type VaultProvider struct{}

func (p *VaultProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Implementation would use Vault API
	// For now, return not available to avoid dependency
	return "", fmt.Errorf("Vault provider not implemented yet")
}

func (p *VaultProvider) Type() string {
	return "vault"
}

func (p *VaultProvider) IsAvailable() bool {
	// Check if Vault is configured
	vaultAddr := os.Getenv("VAULT_ADDR")
	return vaultAddr != ""
}

// ResolvePlaceholders replaces placeholder values in configuration with actual secrets
func (sm *SecretManager) ResolvePlaceholders(ctx context.Context, value string) (string, error) {
	if !strings.Contains(value, "${") {
		return value, nil
	}

	// Simple placeholder resolution for ${SECRET_NAME} format
	start := strings.Index(value, "${")
	if start == -1 {
		return value, nil
	}
	
	end := strings.Index(value[start:], "}")
	if end == -1 {
		return "", fmt.Errorf("malformed placeholder in value")
	}
	
	secretKey := value[start+2 : start+end]
	secretValue, err := sm.GetSecret(ctx, secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to resolve secret %s: %w", secretKey, err)
	}
	
	// Replace the placeholder
	result := value[:start] + secretValue + value[start+end+1:]
	
	// Recursively resolve any remaining placeholders
	return sm.ResolvePlaceholders(ctx, result)
}