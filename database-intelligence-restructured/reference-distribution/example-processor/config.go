package exampleprocessor

// Config represents the receiver config settings
type Config struct{}

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
