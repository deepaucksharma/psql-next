package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type scraper struct {
	logger         *zap.Logger
	config         *Config
	settings       receiver.Settings
	client         redis.UniversalClient
	mb             *metricBuilder
	lastSlowLogID  int64
	clusterEnabled bool
}

func newScraper(settings receiver.Settings, cfg *Config) *scraper {
	return &scraper{
		logger:        settings.Logger,
		config:        cfg,
		settings:      settings,
		mb:            newMetricBuilder(cfg),
		lastSlowLogID: -1,
	}
}

func (s *scraper) start(ctx context.Context, host component.Host) error {
	// Create TLS config if enabled
	var tlsConfig *tls.Config
	if s.config.TLS.Enabled {
		var err error
		tlsConfig, err = s.createTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
	}

	// Create Redis client based on mode
	if s.config.Cluster.Enabled {
		// Cluster mode
		s.clusterEnabled = true
		clusterOpts := &redis.ClusterOptions{
			Addrs:          s.config.Cluster.Nodes,
			Password:       s.config.Password,
			Username:       s.config.Username,
			MaxRetries:     s.config.MaxRetries,
			MaxRedirects:   s.config.Cluster.MaxRedirects,
			ReadOnly:       s.config.Cluster.ReadOnly,
			RouteByLatency: s.config.Cluster.RouteByLatency,
			RouteRandomly:  s.config.Cluster.RouteRandomly,
			DialTimeout:    s.config.ConnectTimeout,
			ReadTimeout:    s.config.ReadTimeout,
			WriteTimeout:   s.config.WriteTimeout,
			PoolSize:       s.config.MaxConns,
			MinIdleConns:   s.config.MinIdleConns,
			PoolTimeout:    s.config.PoolTimeout,
			TLSConfig:      tlsConfig,
		}
		s.client = redis.NewClusterClient(clusterOpts)
	} else if s.config.Sentinel.Enabled {
		// Sentinel mode
		sentinelOpts := &redis.FailoverOptions{
			MasterName:       s.config.Sentinel.MasterName,
			SentinelAddrs:    s.config.Sentinel.SentinelAddrs,
			SentinelPassword: s.config.Sentinel.SentinelPassword,
			Password:         s.config.Password,
			Username:         s.config.Username,
			DB:               s.config.Database,
			MaxRetries:       s.config.MaxRetries,
			DialTimeout:      s.config.ConnectTimeout,
			ReadTimeout:      s.config.ReadTimeout,
			WriteTimeout:     s.config.WriteTimeout,
			PoolSize:         s.config.MaxConns,
			MinIdleConns:     s.config.MinIdleConns,
			PoolTimeout:      s.config.PoolTimeout,
			TLSConfig:        tlsConfig,
		}
		s.client = redis.NewFailoverClient(sentinelOpts)
	} else {
		// Standalone mode
		opts := &redis.Options{
			Addr:         s.config.Endpoint,
			Password:     s.config.Password,
			Username:     s.config.Username,
			DB:           s.config.Database,
			MaxRetries:   s.config.MaxRetries,
			DialTimeout:  s.config.ConnectTimeout,
			ReadTimeout:  s.config.ReadTimeout,
			WriteTimeout: s.config.WriteTimeout,
			PoolSize:     s.config.MaxConns,
			MinIdleConns: s.config.MinIdleConns,
			PoolTimeout:  s.config.PoolTimeout,
			TLSConfig:    tlsConfig,
		}
		s.client = redis.NewClient(opts)
	}

	// Test connection
	if err := s.client.Ping(ctx).Err(); err != nil {
		s.client.Close()
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	s.logger.Info("Connected to Redis",
		zap.String("endpoint", s.config.getEndpointMasked()),
		zap.Bool("cluster", s.config.Cluster.Enabled),
		zap.Bool("sentinel", s.config.Sentinel.Enabled))

	return nil
}

func (s *scraper) shutdown(ctx context.Context) error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics().AppendEmpty()
	rs := rms.Resource()

	// Set resource attributes
	rs.Attributes().PutStr("redis.instance", s.config.Endpoint)
	if s.config.Database > 0 {
		rs.Attributes().PutInt("redis.database", int64(s.config.Database))
	}
	for k, v := range s.config.ResourceAttributes {
		rs.Attributes().PutStr(k, v)
	}

	ilm := rms.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("otelcol/redis")

	now := pcommon.NewTimestampFromTime(time.Now())

	// Collect server info metrics
	if err := s.collectServerInfo(ctx, ilm.Metrics(), now); err != nil {
		s.logger.Error("Failed to collect server info", zap.Error(err))
	}

	// Collect command stats
	if s.config.Metrics.CommandStats {
		if err := s.collectCommandStats(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect command stats", zap.Error(err))
		}
	}

	// Collect latency stats
	if s.config.Metrics.LatencyStats {
		if err := s.collectLatencyStats(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect latency stats", zap.Error(err))
		}
	}

	// Collect memory stats
	if s.config.Metrics.MemoryStats {
		if err := s.collectMemoryStats(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect memory stats", zap.Error(err))
		}
	}

	// Collect client list
	if s.config.Metrics.ClientList {
		if err := s.collectClientList(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect client list", zap.Error(err))
		}
	}

	// Collect slow log
	if s.config.SlowLog.Enabled {
		if err := s.collectSlowLog(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect slow log", zap.Error(err))
		}
	}

	// Collect cluster metrics
	if s.config.Cluster.Enabled {
		if err := s.collectClusterMetrics(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect cluster metrics", zap.Error(err))
		}
	}

	// Collect sentinel metrics
	if s.config.Sentinel.Enabled && s.config.Sentinel.CollectSentinelMetrics {
		if err := s.collectSentinelMetrics(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect sentinel metrics", zap.Error(err))
		}
	}

	// Collect custom metrics
	for _, cmd := range s.config.Metrics.CustomCommands {
		if err := s.collectCustomMetric(ctx, ilm.Metrics(), now, cmd); err != nil {
			s.logger.Error("Failed to collect custom metric",
				zap.String("metric", cmd.Name),
				zap.Error(err))
		}
	}

	return md, nil
}

func (s *scraper) collectServerInfo(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Build INFO command sections
	sections := []string{}
	cfg := s.config.Metrics.ServerInfo
	if cfg.Server {
		sections = append(sections, "server")
	}
	if cfg.Clients {
		sections = append(sections, "clients")
	}
	if cfg.Memory {
		sections = append(sections, "memory")
	}
	if cfg.Persistence {
		sections = append(sections, "persistence")
	}
	if cfg.Stats {
		sections = append(sections, "stats")
	}
	if cfg.Replication {
		sections = append(sections, "replication")
	}
	if cfg.CPU {
		sections = append(sections, "cpu")
	}
	if cfg.Cluster && s.clusterEnabled {
		sections = append(sections, "cluster")
	}
	if cfg.Keyspace {
		sections = append(sections, "keyspace")
	}

	// Get info for requested sections
	var info string
	var err error
	if len(sections) == 0 {
		info, err = s.client.Info(ctx).Result()
	} else {
		info, err = s.client.Info(ctx, sections...).Result()
	}
	if err != nil {
		return fmt.Errorf("INFO command failed: %w", err)
	}

	// Parse and record metrics
	infoMap := parseRedisInfo(info)
	s.mb.recordServerInfo(metrics, infoMap, now)

	// Collect keyspace stats separately if enabled
	if s.config.Metrics.KeyspaceStats && cfg.Keyspace {
		s.mb.recordKeyspaceStats(metrics, infoMap, now)
	}

	return nil
}

func (s *scraper) collectCommandStats(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Get command stats
	stats, err := s.client.Info(ctx, "commandstats").Result()
	if err != nil {
		return fmt.Errorf("INFO commandstats failed: %w", err)
	}

	statsMap := parseRedisInfo(stats)
	s.mb.recordCommandStats(metrics, statsMap, now)
	return nil
}

func (s *scraper) collectLatencyStats(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Get latest latency events
	result, err := s.client.Do(ctx, "LATENCY", "LATEST").Result()
	if err != nil {
		return fmt.Errorf("LATENCY LATEST failed: %w", err)
	}

	if events, ok := result.([]interface{}); ok {
		s.mb.recordLatencyEvents(metrics, events, now)
	}

	// Get latency histogram if available (Redis 7.0+)
	histResult, err := s.client.Do(ctx, "LATENCY", "HISTOGRAM").Result()
	if err == nil {
		if histograms, ok := histResult.(map[string]interface{}); ok {
			s.mb.recordLatencyHistograms(metrics, histograms, now)
		}
	}

	return nil
}

func (s *scraper) collectMemoryStats(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Get memory stats
	result, err := s.client.Do(ctx, "MEMORY", "STATS").Result()
	if err != nil {
		return fmt.Errorf("MEMORY STATS failed: %w", err)
	}

	// Parse memory stats (returns array of key-value pairs)
	if stats, ok := result.([]interface{}); ok {
		memStats := make(map[string]interface{})
		for i := 0; i < len(stats)-1; i += 2 {
			if key, ok := stats[i].(string); ok {
				memStats[key] = stats[i+1]
			}
		}
		s.mb.recordMemoryStats(metrics, memStats, now)
	}

	return nil
}

func (s *scraper) collectClientList(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Get client list
	clients, err := s.client.ClientList(ctx).Result()
	if err != nil {
		return fmt.Errorf("CLIENT LIST failed: %w", err)
	}

	// Parse client list
	clientInfos := parseClientList(clients)
	s.mb.recordClientMetrics(metrics, clientInfos, now)
	return nil
}

func (s *scraper) collectSlowLog(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Get slow log entries
	entries, err := s.client.SlowLogGet(ctx, int64(s.config.SlowLog.MaxEntries)).Result()
	if err != nil {
		return fmt.Errorf("SLOWLOG GET failed: %w", err)
	}

	// Track position if enabled
	if s.config.SlowLog.TrackPosition && len(entries) > 0 {
		// Only process new entries
		newEntries := []redis.SlowLog{}
		for _, entry := range entries {
			if s.lastSlowLogID < 0 || entry.ID > s.lastSlowLogID {
				newEntries = append(newEntries, entry)
			}
		}
		if len(newEntries) > 0 {
			s.lastSlowLogID = entries[0].ID
			entries = newEntries
		}
	}

	s.mb.recordSlowLogEntries(metrics, entries, now, s.config.SlowLog.IncludeCommands)
	return nil
}

func (s *scraper) collectClusterMetrics(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	if !s.clusterEnabled {
		return nil
	}

	// Get cluster info
	if s.config.Cluster.CollectClusterInfo {
		info, err := s.client.ClusterInfo(ctx).Result()
		if err != nil {
			return fmt.Errorf("CLUSTER INFO failed: %w", err)
		}
		clusterInfo := parseRedisInfo(info)
		s.mb.recordClusterInfo(metrics, clusterInfo, now)
	}

	// Get cluster nodes
	nodes, err := s.client.ClusterNodes(ctx).Result()
	if err != nil {
		return fmt.Errorf("CLUSTER NODES failed: %w", err)
	}
	nodeInfos := parseClusterNodes(nodes)
	s.mb.recordClusterNodes(metrics, nodeInfos, now)

	// Get slot distribution if enabled
	if s.config.Cluster.CollectSlotMetrics {
		slots, err := s.client.ClusterSlots(ctx).Result()
		if err != nil {
			return fmt.Errorf("CLUSTER SLOTS failed: %w", err)
		}
		s.mb.recordClusterSlots(metrics, slots, now)
	}

	// Collect per-node metrics if enabled
	if s.config.Cluster.CollectPerNodeMetrics {
		if clusterClient, ok := s.client.(*redis.ClusterClient); ok {
			err := clusterClient.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
				nodeAddr := shard.Options().Addr
				
				// Get node-specific info
				info, err := shard.Info(ctx).Result()
				if err != nil {
					s.logger.Warn("Failed to get info for cluster node",
						zap.String("node", nodeAddr),
						zap.Error(err))
					return nil // Continue with other nodes
				}

				infoMap := parseRedisInfo(info)
				s.mb.recordNodeMetrics(metrics, nodeAddr, infoMap, now)
				return nil
			})
			if err != nil {
				s.logger.Warn("Failed to collect per-node metrics", zap.Error(err))
			}
		}
	}

	return nil
}

func (s *scraper) collectSentinelMetrics(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Create a client to connect to sentinel
	sentinelClient := redis.NewClient(&redis.Options{
		Addr:     s.config.Sentinel.SentinelAddrs[0],
		Password: s.config.Sentinel.SentinelPassword,
	})
	defer sentinelClient.Close()

	// Get sentinel masters
	mastersResult, err := sentinelClient.Do(ctx, "SENTINEL", "masters").Result()
	if err != nil {
		return fmt.Errorf("SENTINEL masters failed: %w", err)
	}

	if masters, ok := mastersResult.([]interface{}); ok {
		s.mb.recordSentinelMasters(metrics, masters, now)
	}

	// Get sentinel info about our master
	masterInfoResult, err := sentinelClient.Do(ctx, "SENTINEL", "master", s.config.Sentinel.MasterName).Result()
	if err != nil {
		return fmt.Errorf("SENTINEL master %s failed: %w", s.config.Sentinel.MasterName, err)
	}

	if masterInfo, ok := masterInfoResult.([]interface{}); ok {
		s.mb.recordSentinelMasterInfo(metrics, s.config.Sentinel.MasterName, masterInfo, now)
	}

	// Get replicas info
	replicasResult, err := sentinelClient.Do(ctx, "SENTINEL", "replicas", s.config.Sentinel.MasterName).Result()
	if err != nil {
		return fmt.Errorf("SENTINEL replicas failed: %w", err)
	}

	if replicas, ok := replicasResult.([]interface{}); ok {
		s.mb.recordSentinelReplicas(metrics, s.config.Sentinel.MasterName, replicas, now)
	}

	return nil
}

func (s *scraper) collectCustomMetric(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp, cmd CustomCommand) error {
	// Execute custom command
	args := make([]interface{}, len(cmd.Args))
	for i, arg := range cmd.Args {
		args[i] = arg
	}

	result, err := s.client.Do(ctx, cmd.Command, args...).Result()
	if err != nil {
		return fmt.Errorf("custom command %s failed: %w", cmd.Command, err)
	}

	s.mb.recordCustomMetric(metrics, cmd, result, now)
	return nil
}

func (s *scraper) createTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: s.config.TLS.Insecure,
		ServerName:         s.config.TLS.ServerName,
	}

	if s.config.TLS.CAFile != "" {
		caCert, err := ioutil.ReadFile(s.config.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	if s.config.TLS.CertFile != "" && s.config.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.config.TLS.CertFile, s.config.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// Helper functions

func parseRedisInfo(info string) map[string]map[string]string {
	result := make(map[string]map[string]string)
	currentSection := ""

	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "# ") {
				currentSection = strings.ToLower(strings.TrimPrefix(line, "# "))
				result[currentSection] = make(map[string]string)
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && currentSection != "" {
			result[currentSection][parts[0]] = parts[1]
		}
	}

	return result
}

func parseClientList(clientList string) []map[string]string {
	clients := []map[string]string{}
	lines := strings.Split(clientList, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		client := make(map[string]string)
		pairs := strings.Split(line, " ")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				client[kv[0]] = kv[1]
			}
		}
		if len(client) > 0 {
			clients = append(clients, client)
		}
	}

	return clients
}

func parseClusterNodes(nodes string) []map[string]string {
	nodeInfos := []map[string]string{}
	lines := strings.Split(nodes, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 8 {
			nodeInfo := map[string]string{
				"id":       parts[0],
				"address":  parts[1],
				"flags":    parts[2],
				"master":   parts[3],
				"ping":     parts[4],
				"pong":     parts[5],
				"epoch":    parts[6],
				"state":    parts[7],
			}
			
			// Parse slots if present
			if len(parts) > 8 {
				slots := strings.Join(parts[8:], " ")
				nodeInfo["slots"] = slots
			}

			nodeInfos = append(nodeInfos, nodeInfo)
		}
	}

	return nodeInfos
}

func parseIntValue(value string) int64 {
	if v, err := strconv.ParseInt(value, 10, 64); err == nil {
		return v
	}
	return 0
}

func parseFloatValue(value string) float64 {
	if v, err := strconv.ParseFloat(value, 64); err == nil {
		return v
	}
	return 0
}