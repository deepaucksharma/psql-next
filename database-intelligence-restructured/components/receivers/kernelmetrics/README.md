# Kernel Metrics Receiver

The Kernel Metrics Receiver uses eBPF (extended Berkeley Packet Filter) to collect low-level kernel metrics from database processes. This provides insights into system calls, I/O operations, CPU usage, and lock contention at the kernel level.

## Overview

This receiver attaches eBPF programs to kernel functions to trace database process activity with minimal overhead. It provides visibility into:

- System call latency and frequency
- File I/O operations and latency
- Network operations
- Memory allocations
- CPU profiling
- Lock contention
- Database-specific operations

## Requirements

- Linux kernel 4.14+ (5.4+ recommended)
- BPF/BTF support enabled in kernel
- CAP_SYS_ADMIN, CAP_SYS_PTRACE, CAP_PERFMON, CAP_BPF capabilities (or root)
- libbpf development headers

## Configuration

```yaml
receivers:
  kernelmetrics:
    # Collection settings
    collection_interval: 10s
    initial_delay: 1s
    
    # eBPF programs to enable
    programs:
      syscall_trace: true    # System call tracing
      file_io_trace: true    # File I/O operations
      network_trace: false   # Network operations
      memory_trace: false    # Memory allocations
      cpu_profile: true      # CPU profiling
      lock_trace: true       # Lock contention
      db_query_trace: true   # Database query tracing
      db_conn_trace: true    # Database connection tracing
    
    # Target process configuration
    target_process:
      process_name: "postgres"     # Process name to monitor
      # pid: 1234                  # Or specific PID
      # cmdline_pattern: "postgres" # Or command line pattern
      follow_children: true        # Monitor child processes
    
    # Resource limits
    cpu_limit: 5.0          # Max 5% CPU usage
    memory_limit_mb: 100    # Max 100MB memory
    
    # Buffer settings
    buffer_size: 8192
    ring_buffer_size: 8388608  # 8MB
```

## Metrics

### System Call Metrics

- `kernel.syscall.count` - Number of system calls by type
- `kernel.syscall.latency` - System call latency distribution
- `kernel.syscall.errors` - System call errors

### File I/O Metrics

- `kernel.file.read.bytes` - Bytes read from files
- `kernel.file.write.bytes` - Bytes written to files
- `kernel.file.read.latency` - File read latency
- `kernel.file.write.latency` - File write latency
- `kernel.file.operations` - File operations by type

### CPU Metrics

- `kernel.cpu.usage` - CPU usage by function
- `kernel.cpu.cycles` - CPU cycles consumed
- `kernel.cpu.cache_misses` - CPU cache misses
- `kernel.cpu.branch_misses` - Branch prediction misses

### Memory Metrics

- `kernel.memory.allocations` - Memory allocations
- `kernel.memory.frees` - Memory deallocations
- `kernel.memory.page_faults` - Page faults
- `kernel.memory.usage` - Memory usage by type

### Lock Metrics

- `kernel.lock.contentions` - Lock contention events
- `kernel.lock.wait_time` - Lock wait time distribution
- `kernel.lock.hold_time` - Lock hold time distribution

### Database-Specific Metrics

- `kernel.db.query.start` - Query start events
- `kernel.db.query.latency` - Query execution latency at kernel level
- `kernel.db.connection.events` - Connection events
- `kernel.db.transaction.events` - Transaction events

## Security Considerations

1. **Capabilities**: The receiver requires elevated privileges. Use Linux capabilities instead of running as root when possible.

2. **Performance Impact**: While eBPF is designed for low overhead, monitoring can still impact performance. Use resource limits and selective tracing.

3. **Kernel Compatibility**: eBPF features vary by kernel version. The receiver will automatically detect and use available features.

## Implementation Notes

This is a stub implementation that demonstrates the structure for a kernel metrics receiver. A full implementation would require:

1. **eBPF Programs**: Written in C and compiled to BPF bytecode
2. **BPF Maps**: For communication between kernel and user space
3. **libbpf Integration**: For loading and managing BPF programs
4. **Process Discovery**: For finding and attaching to database processes
5. **Event Processing**: For efficiently processing kernel events

## Example eBPF Program (Conceptual)

```c
// Example: Trace PostgreSQL query execution
SEC("uprobe/postgres:exec_simple_query")
int trace_query_start(struct pt_regs *ctx) {
    struct query_event event = {};
    
    event.timestamp = bpf_ktime_get_ns();
    event.pid = bpf_get_current_pid_tgid() >> 32;
    
    // Read query string from function arguments
    bpf_probe_read_user_str(&event.query, sizeof(event.query), 
                           (void *)PT_REGS_PARM1(ctx));
    
    // Submit event to ring buffer
    bpf_ringbuf_submit(&event, 0);
    
    return 0;
}
```

## Future Enhancements

1. **Dynamic Tracing**: Allow runtime configuration of trace points
2. **Flame Graphs**: Generate flame graphs from CPU profiling data
3. **Correlation**: Correlate kernel events with application-level metrics
4. **Machine Learning**: Detect anomalies in kernel behavior patterns
5. **Container Support**: Enhanced support for containerized databases