# JMX Receiver - Domain-Driven Design Analysis

## Executive Summary

The JMX receiver implements a sophisticated bridge pattern between Java Management Extensions (JMX) and OpenTelemetry, organized into four primary bounded contexts:

1. **Subprocess Management Context**: Manages Java process lifecycle and reliability
2. **JMX Integration Context**: Handles JMX connections and metric gathering configuration
3. **Security & Validation Context**: Ensures JAR integrity and secure connections
4. **Telemetry Bridge Context**: Transforms JMX metrics into OpenTelemetry format

## Bounded Contexts

### 1. Subprocess Management Context

**Core Responsibility**: Reliable execution and lifecycle management of the Java JMX Metric Gatherer process

**Aggregates**:
- **Subprocess** (Aggregate Root)
  - Manages Java process lifecycle
  - Handles automatic restarts on failure
  - Controls stdin/stdout/stderr communication
  - Enforces process termination guarantees

**Domain Services**:
- **ProcessMonitor**: Tracks subprocess health and triggers restarts
- **RestartPolicy**: Implements restart strategies with delays
- **EnvironmentSanitizer**: Cleans environment variables for security

**Value Objects**:
- **SubprocessConfig**: Immutable configuration for subprocess behavior
- **RestartDelay**: Time delay between restart attempts
- **ProcessExitStatus**: Result of process termination

**Invariants**:
- Subprocess must terminate when parent process dies (Linux)
- Restart delay must increase to prevent rapid cycling
- Environment must be sanitized before subprocess launch

### 2. JMX Integration Context

**Core Responsibility**: Configure and manage JMX connections to target Java applications

**Aggregates**:
- **JMXConfiguration** (Aggregate Root)
  - Manages JMX connection parameters
  - Handles authentication credentials
  - Configures target system selection

**Value Objects**:
- **JMXEndpoint**: Service URL for JMX connection
- **TargetSystem**: Enumeration of supported systems (kafka, cassandra, etc.)
- **CollectionInterval**: Metric gathering frequency
- **GroovyScript**: Target-specific collection scripts

**Domain Services**:
- **PropertyFileBuilder**: Generates Java properties for JMX Gatherer
- **TargetSystemRegistry**: Maps target systems to Groovy scripts

**Invariants**:
- Endpoint must be valid JMX service URL format
- Target system must be in supported list
- Collection interval must be positive duration
- Authentication credentials must be complete if provided

### 3. Security & Validation Context

**Core Responsibility**: Ensure secure execution and validate component integrity

**Aggregates**:
- **JARValidator** (Aggregate Root)
  - Validates JAR file integrity via SHA256
  - Manages supported JAR versions
  - Handles additional JAR dependencies

**Value Objects**:
- **JARFile**: Path and hash of JAR file
- **SupportedJAR**: Validated JAR with version info
- **SSLConfig**: TLS configuration for secure connections
- **AuthenticationConfig**: Credentials and SASL settings

**Domain Services**:
- **HashValidator**: Computes and verifies SHA256 hashes
- **SSLContextBuilder**: Creates secure connection contexts
- **CredentialManager**: Securely handles authentication data

**Invariants**:
- JAR file must exist and match known SHA256 hash
- SSL configuration must be complete if enabled
- Passwords must never be logged or exposed
- Additional JARs must be validated if provided

### 4. Telemetry Bridge Context

**Core Responsibility**: Transform JMX metrics into OpenTelemetry format via internal OTLP receiver

**Aggregates**:
- **OTLPReceiver** (Aggregate Root)
  - Creates internal gRPC server for metrics
  - Manages port allocation and lifecycle
  - Forwards metrics to consumer pipeline

**Value Objects**:
- **OTLPEndpoint**: Local endpoint for metric submission
- **MetricBatch**: Collection of metrics from JMX
- **ResourceAttributes**: Additional metadata for metrics

**Domain Services**:
- **PortAllocator**: Finds available ports for internal receiver
- **MetricForwarder**: Routes metrics to configured consumer
- **HeaderProcessor**: Handles additional OTLP headers

**Invariants**:
- Internal receiver must use localhost only
- Port must be dynamically allocated to avoid conflicts
- All metrics must preserve resource attributes
- Consumer must receive all successfully parsed metrics

## Domain Events

### Subprocess Events
- `SubprocessStarting`: Java process launch initiated
- `SubprocessStarted`: Process successfully started with PID
- `SubprocessFailed`: Process exited unexpectedly
- `SubprocessRestarting`: Automatic restart triggered
- `SubprocessShutdown`: Graceful termination completed

### JMX Events
- `JMXConnectionConfigured`: Properties file generated
- `JMXConnectionEstablished`: Successfully connected to target
- `JMXConnectionFailed`: Unable to reach JMX endpoint
- `JMXAuthenticationFailed`: Credentials rejected

### Security Events
- `JARValidationStarted`: Beginning hash verification
- `JARValidationPassed`: JAR integrity confirmed
- `JARValidationFailed`: Hash mismatch detected
- `SSLHandshakeCompleted`: Secure connection established

### Telemetry Events
- `OTLPReceiverStarted`: Internal receiver ready
- `MetricBatchReceived`: Metrics received from subprocess
- `MetricBatchForwarded`: Metrics sent to consumer
- `OTLPReceiverStopped`: Shutdown completed

## Anti-Patterns Addressed

1. **No Direct JMX Integration**: Uses subprocess to isolate Java dependencies
2. **No Static Port Binding**: Dynamic port allocation prevents conflicts
3. **No Unvalidated Execution**: All JARs verified before running
4. **No Credential Exposure**: Passwords isolated in properties file
5. **No Zombie Processes**: Parent death signal ensures cleanup

## Architectural Patterns

### 1. Bridge Pattern
```go
// JMX Receiver bridges JMX world to OpenTelemetry
type jmxMetricReceiver struct {
    subprocess      *subprocess.Subprocess  // Java side
    otlpReceiver    receiver.Metrics       // OTel side
}
```

### 2. Factory Pattern
```go
func (f *jmxReceiverFactory) createMetricsReceiver(
    ctx context.Context,
    params receiver.CreateSettings,
    cfg component.Config,
    consumer consumer.Metrics,
) (receiver.Metrics, error)
```

### 3. Strategy Pattern
```go
// Different target systems use different Groovy scripts
type TargetSystem string

const (
    TargetSystemCassandra TargetSystem = "cassandra"
    TargetSystemKafka     TargetSystem = "kafka"
    // ...
)
```

### 4. Proxy Pattern
```go
// Internal OTLP receiver proxies metrics from subprocess
func (r *jmxMetricReceiver) buildOTLPReceiver() (receiver.Metrics, error)
```

## Aggregate Invariants Detail

### Subprocess Aggregate
1. **Lifecycle Invariants**
   - Must not start if already running
   - Must terminate within shutdown timeout
   - Must clean up resources on failure
   - Restart count must be tracked

2. **Communication Invariants**
   - Stdout/stderr must be continuously drained
   - Log level must be propagated to subprocess
   - Environment must exclude sensitive variables

### JARValidator Aggregate
1. **Integrity Invariants**
   - SHA256 hash must match exactly
   - File must be readable and executable
   - Version must be tracked for telemetry
   - Custom JARs require explicit override

2. **Dependency Invariants**
   - Additional JARs validated if provided
   - Classpath order must be maintained
   - Missing dependencies must fail fast

## Testing Strategy

### Unit Testing
- Mock subprocess for receiver tests
- Test property file generation
- Validate configuration parsing
- Test hash validation logic

### Integration Testing
- Test against real Java applications
- Validate metric collection from each target
- Test authentication scenarios
- Verify SSL connections

### Chaos Testing
- Subprocess crash recovery
- Network disconnection handling
- Invalid JAR handling
- Port allocation under load

## Performance Considerations

1. **Process Overhead**: Single subprocess per receiver instance
2. **Memory Usage**: JVM heap configured via JAVA_OPTS
3. **Collection Interval**: Configurable to balance load
4. **Metric Batching**: Handled by OTLP protocol
5. **Restart Delays**: Exponential backoff prevents thrashing

## Security Model

1. **JAR Validation**: SHA256 verification prevents tampering
2. **Process Isolation**: Subprocess runs with sanitized environment
3. **Authentication**: Username/password, Kerberos SASL support
4. **Encryption**: SSL/TLS for JMX connections
5. **Port Security**: Internal receiver binds to localhost only

## Evolution Points

1. **New Target Systems**: Add Groovy scripts and configuration
2. **Authentication Methods**: Extend SASL profile support
3. **JAR Updates**: Update hash list for new versions
4. **Metric Extensions**: Modify Groovy scripts for new metrics
5. **Platform Support**: Extend subprocess management

## Error Handling Strategy

1. **Fail Fast**: Invalid configuration stops startup
2. **Graceful Degradation**: Subprocess failures trigger restarts
3. **Clear Diagnostics**: Detailed error messages for troubleshooting
4. **Partial Failures**: Continue operation if possible
5. **Resource Cleanup**: Always clean up on shutdown

## Conclusion

The JMX receiver demonstrates sophisticated DDD principles:
- Clear separation between Java/JMX and OpenTelemetry domains
- Strong security model with validation at boundaries
- Resilient subprocess management with automatic recovery
- Clean abstractions enabling testing and evolution
- Bridge pattern effectively connects two technology ecosystems

The architecture provides production-ready JMX metric collection while maintaining security, reliability, and operational simplicity.