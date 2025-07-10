package kernelmetrics

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// Config represents the receiver configuration for kernel-level metrics
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	
	// eBPF programs to enable
	Programs ProgramsConfig `mapstructure:"programs"`
	
	// Target process settings
	TargetProcess ProcessConfig `mapstructure:"target_process"`
	
	// Collection settings
	BufferSize     int           `mapstructure:"buffer_size"`
	RingBufferSize int           `mapstructure:"ring_buffer_size"`
	
	// Performance settings
	CPULimit      float64       `mapstructure:"cpu_limit"`      // Max CPU usage percentage
	MemoryLimitMB int           `mapstructure:"memory_limit_mb"` // Max memory usage
	
	// Security settings
	RequireRoot   bool          `mapstructure:"require_root"`
	Capabilities  []string      `mapstructure:"capabilities"`
}

// ProgramsConfig specifies which eBPF programs to enable
type ProgramsConfig struct {
	// System call tracing
	SyscallTrace   bool `mapstructure:"syscall_trace"`
	
	// File I/O tracing
	FileIOTrace    bool `mapstructure:"file_io_trace"`
	
	// Network tracing
	NetworkTrace   bool `mapstructure:"network_trace"`
	
	// Memory allocation tracing
	MemoryTrace    bool `mapstructure:"memory_trace"`
	
	// CPU profiling
	CPUProfile     bool `mapstructure:"cpu_profile"`
	
	// Lock contention tracing
	LockTrace      bool `mapstructure:"lock_trace"`
	
	// Database-specific tracing
	DBQueryTrace   bool `mapstructure:"db_query_trace"`
	DBConnTrace    bool `mapstructure:"db_conn_trace"`
}

// ProcessConfig specifies how to identify the target process
type ProcessConfig struct {
	// Process name pattern (e.g., "postgres", "mysql")
	ProcessName    string `mapstructure:"process_name"`
	
	// Process ID (if known)
	PID            int    `mapstructure:"pid"`
	
	// Command line pattern
	CmdlinePattern string `mapstructure:"cmdline_pattern"`
	
	// Follow child processes
	FollowChildren bool   `mapstructure:"follow_children"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	// Validate target process settings
	if cfg.TargetProcess.ProcessName == "" && cfg.TargetProcess.PID == 0 && cfg.TargetProcess.CmdlinePattern == "" {
		return errors.New("at least one of process_name, pid, or cmdline_pattern must be specified")
	}
	
	// Validate that at least one program is enabled
	if !cfg.Programs.SyscallTrace &&
		!cfg.Programs.FileIOTrace &&
		!cfg.Programs.NetworkTrace &&
		!cfg.Programs.MemoryTrace &&
		!cfg.Programs.CPUProfile &&
		!cfg.Programs.LockTrace &&
		!cfg.Programs.DBQueryTrace &&
		!cfg.Programs.DBConnTrace {
		return errors.New("at least one eBPF program must be enabled")
	}
	
	// Set defaults
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 8192
	}
	
	if cfg.RingBufferSize <= 0 {
		cfg.RingBufferSize = 8 * 1024 * 1024 // 8MB
	}
	
	if cfg.CPULimit <= 0 {
		cfg.CPULimit = 5.0 // 5% CPU max
	}
	
	if cfg.MemoryLimitMB <= 0 {
		cfg.MemoryLimitMB = 100 // 100MB max
	}
	
	return nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
		},
		Programs: ProgramsConfig{
			SyscallTrace:  true,
			FileIOTrace:   true,
			NetworkTrace:  false,
			MemoryTrace:   false,
			CPUProfile:    true,
			LockTrace:     true,
			DBQueryTrace:  true,
			DBConnTrace:   true,
		},
		TargetProcess: ProcessConfig{
			ProcessName:    "postgres",
			FollowChildren: true,
		},
		BufferSize:     8192,
		RingBufferSize: 8 * 1024 * 1024,
		CPULimit:       5.0,
		MemoryLimitMB:  100,
		RequireRoot:    false, // Try to use capabilities instead
		Capabilities: []string{
			"CAP_SYS_ADMIN",
			"CAP_SYS_PTRACE",
			"CAP_PERFMON",
			"CAP_BPF",
		},
	}
}