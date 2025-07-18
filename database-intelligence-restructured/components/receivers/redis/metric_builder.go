package redis

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricBuilder struct {
	config *Config
}

func newMetricBuilder(cfg *Config) *metricBuilder {
	return &metricBuilder{
		config: cfg,
	}
}

func (mb *metricBuilder) recordServerInfo(metrics pmetric.MetricSlice, info map[string]map[string]string, now pcommon.Timestamp) {
	// Server metrics
	if server, ok := info["server"]; ok {
		mb.addGaugeMetric(metrics, "redis.server.uptime", "Server uptime in seconds",
			"s", parseIntValue(server["uptime_in_seconds"]), now, nil)
	}

	// Clients metrics
	if clients, ok := info["clients"]; ok {
		mb.addGaugeMetric(metrics, "redis.clients.connected", "Number of connected clients",
			"clients", parseIntValue(clients["connected_clients"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.clients.blocked", "Number of blocked clients",
			"clients", parseIntValue(clients["blocked_clients"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.clients.max_input_buffer", "Biggest input buffer among current clients",
			"By", parseIntValue(clients["client_recent_max_input_buffer"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.clients.max_output_buffer", "Biggest output buffer among current clients",
			"By", parseIntValue(clients["client_recent_max_output_buffer"]), now, nil)
	}

	// Memory metrics
	if memory, ok := info["memory"]; ok {
		mb.addGaugeMetric(metrics, "redis.memory.used", "Total memory used",
			"By", parseIntValue(memory["used_memory"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.memory.rss", "Resident set size",
			"By", parseIntValue(memory["used_memory_rss"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.memory.peak", "Peak memory usage",
			"By", parseIntValue(memory["used_memory_peak"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.memory.lua", "Memory used by Lua",
			"By", parseIntValue(memory["used_memory_lua"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.memory.fragmentation_ratio", "Memory fragmentation ratio",
			"ratio", parseFloatValue(memory["mem_fragmentation_ratio"]), now, nil)
		
		// Eviction metrics
		mb.addSumMetric(metrics, "redis.evicted_keys", "Number of evicted keys",
			"keys", parseIntValue(memory["evicted_keys"]), now, nil)
		
		if maxmemory := parseIntValue(memory["maxmemory"]); maxmemory > 0 {
			mb.addGaugeMetric(metrics, "redis.memory.max", "Max memory configuration",
				"By", maxmemory, now, nil)
		}
	}

	// Persistence metrics
	if persistence, ok := info["persistence"]; ok {
		mb.addGaugeMetric(metrics, "redis.persistence.rdb.changes_since_last_save", "Changes since last RDB save",
			"changes", parseIntValue(persistence["rdb_changes_since_last_save"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.persistence.rdb.last_save_time", "Unix timestamp of last RDB save",
			"timestamp", parseIntValue(persistence["rdb_last_save_time"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.persistence.rdb.saves_in_progress", "RDB saves in progress",
			"saves", parseIntValue(persistence["rdb_bgsave_in_progress"]), now, nil)
		
		// AOF metrics
		if aofEnabled := persistence["aof_enabled"]; aofEnabled == "1" {
			mb.addGaugeMetric(metrics, "redis.persistence.aof.enabled", "AOF enabled",
				"bool", 1, now, nil)
			mb.addGaugeMetric(metrics, "redis.persistence.aof.rewrite_in_progress", "AOF rewrite in progress",
				"bool", parseIntValue(persistence["aof_rewrite_in_progress"]), now, nil)
			mb.addGaugeMetric(metrics, "redis.persistence.aof.current_size", "AOF current file size",
				"By", parseIntValue(persistence["aof_current_size"]), now, nil)
			mb.addGaugeMetric(metrics, "redis.persistence.aof.base_size", "AOF base file size",
				"By", parseIntValue(persistence["aof_base_size"]), now, nil)
		} else {
			mb.addGaugeMetric(metrics, "redis.persistence.aof.enabled", "AOF enabled",
				"bool", 0, now, nil)
		}
	}

	// Stats metrics
	if stats, ok := info["stats"]; ok {
		mb.addSumMetric(metrics, "redis.connections.received", "Total connections received",
			"connections", parseIntValue(stats["total_connections_received"]), now, nil)
		mb.addSumMetric(metrics, "redis.commands.processed", "Total commands processed",
			"commands", parseIntValue(stats["total_commands_processed"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.commands.per_second", "Instantaneous ops per second",
			"ops/s", parseIntValue(stats["instantaneous_ops_per_sec"]), now, nil)
		mb.addSumMetric(metrics, "redis.net.input.bytes", "Total network input bytes",
			"By", parseIntValue(stats["total_net_input_bytes"]), now, nil)
		mb.addSumMetric(metrics, "redis.net.output.bytes", "Total network output bytes",
			"By", parseIntValue(stats["total_net_output_bytes"]), now, nil)
		mb.addSumMetric(metrics, "redis.rejected_connections", "Rejected connections",
			"connections", parseIntValue(stats["rejected_connections"]), now, nil)
		mb.addSumMetric(metrics, "redis.expired_keys", "Total expired keys",
			"keys", parseIntValue(stats["expired_keys"]), now, nil)
		mb.addGaugeMetric(metrics, "redis.keyspace.hits.ratio", "Keyspace hit ratio",
			"ratio", calculateHitRatio(stats["keyspace_hits"], stats["keyspace_misses"]), now, nil)
		mb.addSumMetric(metrics, "redis.keyspace.hits", "Keyspace hits",
			"hits", parseIntValue(stats["keyspace_hits"]), now, nil)
		mb.addSumMetric(metrics, "redis.keyspace.misses", "Keyspace misses",
			"misses", parseIntValue(stats["keyspace_misses"]), now, nil)
	}

	// Replication metrics
	if replication, ok := info["replication"]; ok {
		role := replication["role"]
		mb.addInfoMetric(metrics, "redis.replication.role", "Replication role",
			now, map[string]string{"role": role})

		if role == "master" {
			mb.addGaugeMetric(metrics, "redis.replication.connected_slaves", "Connected slaves",
				"slaves", parseIntValue(replication["connected_slaves"]), now, nil)
			
			// Parse slave info
			for i := 0; ; i++ {
				slaveKey := fmt.Sprintf("slave%d", i)
				if slaveInfo, exists := replication[slaveKey]; exists {
					mb.parseAndRecordSlaveInfo(metrics, i, slaveInfo, now)
				} else {
					break
				}
			}
		} else if role == "slave" {
			masterHost := replication["master_host"]
			masterPort := replication["master_port"]
			attrs := map[string]string{
				"master_host": masterHost,
				"master_port": masterPort,
			}
			
			linkStatus := replication["master_link_status"]
			linkUp := 0
			if linkStatus == "up" {
				linkUp = 1
			}
			mb.addGaugeMetric(metrics, "redis.replication.master_link_up", "Master link status",
				"bool", linkUp, now, attrs)
			
			mb.addGaugeMetric(metrics, "redis.replication.master_last_io_seconds", "Seconds since last master IO",
				"s", parseIntValue(replication["master_last_io_seconds_ago"]), now, attrs)
			
			if replOffset := parseIntValue(replication["slave_repl_offset"]); replOffset > 0 {
				mb.addGaugeMetric(metrics, "redis.replication.slave_offset", "Slave replication offset",
					"offset", replOffset, now, attrs)
			}
		}
	}

	// CPU metrics
	if cpu, ok := info["cpu"]; ok {
		mb.addSumMetric(metrics, "redis.cpu.system", "System CPU consumed",
			"s", parseFloatValue(cpu["used_cpu_sys"]), now, nil)
		mb.addSumMetric(metrics, "redis.cpu.user", "User CPU consumed",
			"s", parseFloatValue(cpu["used_cpu_user"]), now, nil)
		mb.addSumMetric(metrics, "redis.cpu.children.system", "System CPU consumed by children",
			"s", parseFloatValue(cpu["used_cpu_sys_children"]), now, nil)
		mb.addSumMetric(metrics, "redis.cpu.children.user", "User CPU consumed by children",
			"s", parseFloatValue(cpu["used_cpu_user_children"]), now, nil)
	}
}

func (mb *metricBuilder) recordKeyspaceStats(metrics pmetric.MetricSlice, info map[string]map[string]string, now pcommon.Timestamp) {
	for section, data := range info {
		if strings.HasPrefix(section, "keyspace") && section != "keyspace" {
			// Extract db number from section name (e.g., "db0" from "keyspace:db0")
			dbName := strings.TrimPrefix(section, "keyspace:")
			if dbName == section {
				continue
			}

			attrs := map[string]string{"database": dbName}

			// Parse keyspace info: keys=1,expires=0,avg_ttl=0
			for _, pair := range strings.Split(data[""], ",") {
				kv := strings.Split(pair, "=")
				if len(kv) == 2 {
					switch kv[0] {
					case "keys":
						mb.addGaugeMetric(metrics, "redis.keyspace.keys", "Number of keys",
							"keys", parseIntValue(kv[1]), now, attrs)
					case "expires":
						mb.addGaugeMetric(metrics, "redis.keyspace.expires", "Number of keys with expiration",
							"keys", parseIntValue(kv[1]), now, attrs)
					case "avg_ttl":
						mb.addGaugeMetric(metrics, "redis.keyspace.avg_ttl", "Average TTL",
							"ms", parseIntValue(kv[1]), now, attrs)
					}
				}
			}
		}
	}
}

func (mb *metricBuilder) recordCommandStats(metrics pmetric.MetricSlice, stats map[string]map[string]string, now pcommon.Timestamp) {
	for section, data := range stats {
		if strings.HasPrefix(section, "cmdstat_") {
			cmdName := strings.TrimPrefix(section, "cmdstat_")
			attrs := map[string]string{"command": cmdName}

			// Parse command stats: calls=1,usec=25,usec_per_call=25.00
			for _, pair := range strings.Split(data[""], ",") {
				kv := strings.Split(pair, "=")
				if len(kv) == 2 {
					switch kv[0] {
					case "calls":
						mb.addSumMetric(metrics, "redis.commands.calls", "Command call count",
							"calls", parseIntValue(kv[1]), now, attrs)
					case "usec":
						mb.addSumMetric(metrics, "redis.commands.usec", "Total time spent",
							"us", parseIntValue(kv[1]), now, attrs)
					case "usec_per_call":
						mb.addGaugeMetric(metrics, "redis.commands.usec_per_call", "Average time per call",
							"us", parseFloatValue(kv[1]), now, attrs)
					}
				}
			}
		}
	}
}

func (mb *metricBuilder) recordLatencyEvents(metrics pmetric.MetricSlice, events []interface{}, now pcommon.Timestamp) {
	for _, event := range events {
		if eventData, ok := event.([]interface{}); ok && len(eventData) >= 2 {
			eventName, _ := eventData[0].(string)
			if latencyData, ok := eventData[1].([]interface{}); ok && len(latencyData) >= 2 {
				timestamp, _ := latencyData[0].(int64)
				latency, _ := latencyData[1].(int64)
				
				attrs := map[string]string{"event": eventName}
				mb.addGaugeMetric(metrics, "redis.latency.latest", "Latest latency for event",
					"ms", latency, now, attrs)
				mb.addGaugeMetric(metrics, "redis.latency.timestamp", "Timestamp of latest event",
					"timestamp", timestamp, now, attrs)
			}
		}
	}
}

func (mb *metricBuilder) recordLatencyHistograms(metrics pmetric.MetricSlice, histograms map[string]interface{}, now pcommon.Timestamp) {
	for event, histogram := range histograms {
		if histData, ok := histogram.(map[string]interface{}); ok {
			attrs := map[string]string{"event": event}
			
			// Extract percentiles
			if p50, ok := histData["p50"].(float64); ok {
				mb.addGaugeMetric(metrics, "redis.latency.percentile", "Latency percentile",
					"ms", p50, now, mergeMaps(attrs, map[string]string{"percentile": "50"}))
			}
			if p99, ok := histData["p99"].(float64); ok {
				mb.addGaugeMetric(metrics, "redis.latency.percentile", "Latency percentile",
					"ms", p99, now, mergeMaps(attrs, map[string]string{"percentile": "99"}))
			}
			if p999, ok := histData["p99.9"].(float64); ok {
				mb.addGaugeMetric(metrics, "redis.latency.percentile", "Latency percentile",
					"ms", p999, now, mergeMaps(attrs, map[string]string{"percentile": "99.9"}))
			}
		}
	}
}

func (mb *metricBuilder) recordMemoryStats(metrics pmetric.MetricSlice, stats map[string]interface{}, now pcommon.Timestamp) {
	for key, value := range stats {
		switch key {
		case "peak.allocated":
			mb.addGaugeMetric(metrics, "redis.memory.peak_allocated", "Peak allocated memory",
				"By", value, now, nil)
		case "total.allocated":
			mb.addGaugeMetric(metrics, "redis.memory.total_allocated", "Total allocated memory",
				"By", value, now, nil)
		case "startup.allocated":
			mb.addGaugeMetric(metrics, "redis.memory.startup_allocated", "Startup allocated memory",
				"By", value, now, nil)
		case "replication.backlog":
			mb.addGaugeMetric(metrics, "redis.memory.replication_backlog", "Replication backlog size",
				"By", value, now, nil)
		case "clients.normal":
			mb.addGaugeMetric(metrics, "redis.memory.clients_normal", "Memory used by normal clients",
				"By", value, now, nil)
		case "clients.slaves":
			mb.addGaugeMetric(metrics, "redis.memory.clients_slaves", "Memory used by replica clients",
				"By", value, now, nil)
		case "aof.buffer":
			mb.addGaugeMetric(metrics, "redis.memory.aof_buffer", "AOF buffer size",
				"By", value, now, nil)
		case "db.0":
			// Database specific memory can be handled separately
			if dbStats, ok := value.(map[string]interface{}); ok {
				mb.recordDatabaseMemoryStats(metrics, 0, dbStats, now)
			}
		}
	}
}

func (mb *metricBuilder) recordDatabaseMemoryStats(metrics pmetric.MetricSlice, db int, stats map[string]interface{}, now pcommon.Timestamp) {
	attrs := map[string]string{"database": fmt.Sprintf("db%d", db)}
	
	if overhead, ok := stats["overhead.hashtable.main"].(int64); ok {
		mb.addGaugeMetric(metrics, "redis.memory.db.overhead", "Database overhead",
			"By", overhead, now, attrs)
	}
	if expires, ok := stats["overhead.hashtable.expires"].(int64); ok {
		mb.addGaugeMetric(metrics, "redis.memory.db.expires_overhead", "Expires overhead",
			"By", expires, now, attrs)
	}
}

func (mb *metricBuilder) recordClientMetrics(metrics pmetric.MetricSlice, clients []map[string]string, now pcommon.Timestamp) {
	// Aggregate client metrics
	clientsByType := make(map[string]int)
	clientsByFlag := make(map[string]int)
	totalInputBuffer := int64(0)
	totalOutputBuffer := int64(0)
	
	for _, client := range clients {
		// Count by client type
		if cmd := client["cmd"]; cmd != "" {
			clientsByType[cmd]++
		}
		
		// Count by flags
		if flags := client["flags"]; flags != "" {
			for _, flag := range strings.Split(flags, "") {
				clientsByFlag[flag]++
			}
		}
		
		// Sum buffers
		if qbuf := client["qbuf"]; qbuf != "" {
			totalInputBuffer += parseIntValue(qbuf)
		}
		if oll := client["oll"]; oll != "" {
			totalOutputBuffer += parseIntValue(oll)
		}
	}
	
	// Record aggregated metrics
	for cmd, count := range clientsByType {
		attrs := map[string]string{"command": cmd}
		mb.addGaugeMetric(metrics, "redis.clients.by_command", "Clients by last command",
			"clients", count, now, attrs)
	}
	
	for flag, count := range clientsByFlag {
		attrs := map[string]string{"flag": flag}
		mb.addGaugeMetric(metrics, "redis.clients.by_flag", "Clients by flag",
			"clients", count, now, attrs)
	}
	
	mb.addGaugeMetric(metrics, "redis.clients.total_input_buffer", "Total client input buffer",
		"By", totalInputBuffer, now, nil)
	mb.addGaugeMetric(metrics, "redis.clients.total_output_buffer", "Total client output buffer",
		"By", totalOutputBuffer, now, nil)
}

func (mb *metricBuilder) recordSlowLogEntries(metrics pmetric.MetricSlice, entries []redis.SlowLog, now pcommon.Timestamp, includeCommands bool) {
	// Record slow log metrics
	mb.addGaugeMetric(metrics, "redis.slowlog.count", "Number of slow log entries",
		"entries", len(entries), now, nil)
	
	if len(entries) > 0 {
		// Find max duration
		maxDuration := int64(0)
		for _, entry := range entries {
			if entry.Duration.Microseconds() > maxDuration {
				maxDuration = entry.Duration.Microseconds()
			}
		}
		mb.addGaugeMetric(metrics, "redis.slowlog.max_duration", "Maximum slow log duration",
			"us", maxDuration, now, nil)
		
		// Record per-command slow log counts if enabled
		if includeCommands {
			cmdCounts := make(map[string]int)
			for _, entry := range entries {
				if len(entry.Args) > 0 {
					cmd := strings.ToUpper(entry.Args[0])
					cmdCounts[cmd]++
				}
			}
			
			for cmd, count := range cmdCounts {
				attrs := map[string]string{"command": cmd}
				mb.addGaugeMetric(metrics, "redis.slowlog.by_command", "Slow log entries by command",
					"entries", count, now, attrs)
			}
		}
	}
}

func (mb *metricBuilder) recordClusterInfo(metrics pmetric.MetricSlice, info map[string]string, now pcommon.Timestamp) {
	state := info["cluster_state"]
	stateOk := 0
	if state == "ok" {
		stateOk = 1
	}
	mb.addGaugeMetric(metrics, "redis.cluster.state", "Cluster state (1=ok, 0=fail)",
		"bool", stateOk, now, nil)
	
	mb.addGaugeMetric(metrics, "redis.cluster.slots_assigned", "Assigned slots",
		"slots", parseIntValue(info["cluster_slots_assigned"]), now, nil)
	mb.addGaugeMetric(metrics, "redis.cluster.slots_ok", "Slots in ok state",
		"slots", parseIntValue(info["cluster_slots_ok"]), now, nil)
	mb.addGaugeMetric(metrics, "redis.cluster.slots_fail", "Slots in fail state",
		"slots", parseIntValue(info["cluster_slots_fail"]), now, nil)
	mb.addGaugeMetric(metrics, "redis.cluster.known_nodes", "Known nodes",
		"nodes", parseIntValue(info["cluster_known_nodes"]), now, nil)
	mb.addGaugeMetric(metrics, "redis.cluster.size", "Number of master nodes",
		"nodes", parseIntValue(info["cluster_size"]), now, nil)
}

func (mb *metricBuilder) recordClusterNodes(metrics pmetric.MetricSlice, nodes []map[string]string, now pcommon.Timestamp) {
	masterCount := 0
	slaveCount := 0
	
	for _, node := range nodes {
		flags := node["flags"]
		if strings.Contains(flags, "master") {
			masterCount++
		}
		if strings.Contains(flags, "slave") {
			slaveCount++
		}
		
		// Record per-node status
		nodeAddr := node["address"]
		attrs := map[string]string{
			"node": nodeAddr,
			"flags": flags,
		}
		
		// Node connection state
		state := node["state"]
		connected := 0
		if state == "connected" {
			connected = 1
		}
		mb.addGaugeMetric(metrics, "redis.cluster.node.connected", "Node connection state",
			"bool", connected, now, attrs)
		
		// Ping/Pong times
		if ping := node["ping"]; ping != "-" {
			mb.addGaugeMetric(metrics, "redis.cluster.node.ping_sent", "Milliseconds since ping sent",
				"ms", parseIntValue(ping), now, attrs)
		}
		if pong := node["pong"]; pong != "-" {
			mb.addGaugeMetric(metrics, "redis.cluster.node.pong_received", "Milliseconds since pong received",
				"ms", parseIntValue(pong), now, attrs)
		}
	}
	
	mb.addGaugeMetric(metrics, "redis.cluster.masters", "Number of master nodes",
		"nodes", masterCount, now, nil)
	mb.addGaugeMetric(metrics, "redis.cluster.slaves", "Number of slave nodes",
		"nodes", slaveCount, now, nil)
}

func (mb *metricBuilder) recordClusterSlots(metrics pmetric.MetricSlice, slots []redis.ClusterSlot, now pcommon.Timestamp) {
	slotsByNode := make(map[string]int)
	
	for _, slot := range slots {
		for _, node := range slot.Nodes {
			nodeKey := fmt.Sprintf("%s:%d", node.Addr, node.ID)
			slotsByNode[nodeKey] += int(slot.End - slot.Start + 1)
		}
	}
	
	for node, count := range slotsByNode {
		attrs := map[string]string{"node": node}
		mb.addGaugeMetric(metrics, "redis.cluster.node.slots", "Slots assigned to node",
			"slots", count, now, attrs)
	}
}

func (mb *metricBuilder) recordNodeMetrics(metrics pmetric.MetricSlice, nodeAddr string, info map[string]map[string]string, now pcommon.Timestamp) {
	nodeAttrs := map[string]string{"node": nodeAddr}
	
	// Record basic node metrics
	if stats, ok := info["stats"]; ok {
		mb.addGaugeMetric(metrics, "redis.cluster.node.ops_per_sec", "Operations per second",
			"ops/s", parseIntValue(stats["instantaneous_ops_per_sec"]), now, nodeAttrs)
	}
	
	if clients, ok := info["clients"]; ok {
		mb.addGaugeMetric(metrics, "redis.cluster.node.connected_clients", "Connected clients",
			"clients", parseIntValue(clients["connected_clients"]), now, nodeAttrs)
	}
	
	if memory, ok := info["memory"]; ok {
		mb.addGaugeMetric(metrics, "redis.cluster.node.used_memory", "Used memory",
			"By", parseIntValue(memory["used_memory"]), now, nodeAttrs)
	}
}

func (mb *metricBuilder) recordSentinelMasters(metrics pmetric.MetricSlice, masters []interface{}, now pcommon.Timestamp) {
	mb.addGaugeMetric(metrics, "redis.sentinel.masters", "Number of monitored masters",
		"masters", len(masters), now, nil)
	
	for _, master := range masters {
		if masterInfo, ok := master.([]interface{}); ok {
			masterData := parseArrayToMap(masterInfo)
			masterName := masterData["name"]
			
			attrs := map[string]string{"master": masterName}
			
			// Master status
			status := masterData["flags"]
			isDown := 0
			if strings.Contains(status, "down") {
				isDown = 1
			}
			mb.addGaugeMetric(metrics, "redis.sentinel.master.down", "Master down status",
				"bool", isDown, now, attrs)
			
			// Sentinel count
			mb.addGaugeMetric(metrics, "redis.sentinel.master.sentinels", "Number of sentinels monitoring",
				"sentinels", parseIntValue(masterData["num-sentinels"]), now, attrs)
			
			// Slave count
			mb.addGaugeMetric(metrics, "redis.sentinel.master.slaves", "Number of slaves",
				"slaves", parseIntValue(masterData["num-slaves"]), now, attrs)
		}
	}
}

func (mb *metricBuilder) recordSentinelMasterInfo(metrics pmetric.MetricSlice, masterName string, info []interface{}, now pcommon.Timestamp) {
	masterData := parseArrayToMap(info)
	attrs := map[string]string{
		"master": masterName,
		"address": masterData["ip"] + ":" + masterData["port"],
	}
	
	// Quorum
	mb.addGaugeMetric(metrics, "redis.sentinel.master.quorum", "Configured quorum",
		"sentinels", parseIntValue(masterData["quorum"]), now, attrs)
	
	// Link status
	linkUp := 0
	if masterData["link-pending-commands"] == "0" {
		linkUp = 1
	}
	mb.addGaugeMetric(metrics, "redis.sentinel.master.link_up", "Master link status",
		"bool", linkUp, now, attrs)
	
	// Last ping
	mb.addGaugeMetric(metrics, "redis.sentinel.master.last_ping", "Milliseconds since last ping",
		"ms", parseIntValue(masterData["last-ping-sent"]), now, attrs)
}

func (mb *metricBuilder) recordSentinelReplicas(metrics pmetric.MetricSlice, masterName string, replicas []interface{}, now pcommon.Timestamp) {
	for _, replica := range replicas {
		if replicaInfo, ok := replica.([]interface{}); ok {
			replicaData := parseArrayToMap(replicaInfo)
			
			attrs := map[string]string{
				"master": masterName,
				"replica": replicaData["name"],
				"address": replicaData["ip"] + ":" + replicaData["port"],
			}
			
			// Replica status
			flags := replicaData["flags"]
			isDown := 0
			if strings.Contains(flags, "down") {
				isDown = 1
			}
			mb.addGaugeMetric(metrics, "redis.sentinel.replica.down", "Replica down status",
				"bool", isDown, now, attrs)
			
			// Link status
			linkUp := 0
			if replicaData["master-link-status"] == "ok" {
				linkUp = 1
			}
			mb.addGaugeMetric(metrics, "redis.sentinel.replica.link_up", "Replica link status",
				"bool", linkUp, now, attrs)
			
			// Lag
			mb.addGaugeMetric(metrics, "redis.sentinel.replica.lag", "Replica lag",
				"s", parseIntValue(replicaData["slave-repl-offset"]), now, attrs)
		}
	}
}

func (mb *metricBuilder) recordCustomMetric(metrics pmetric.MetricSlice, cmd CustomCommand, result interface{}, now pcommon.Timestamp) {
	// Extract value based on type
	var value interface{}
	switch v := result.(type) {
	case int64:
		value = v
	case string:
		// Try to parse as number
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			value = i
		} else if f, err := strconv.ParseFloat(v, 64); err == nil {
			value = f
		} else {
			return // Can't extract numeric value
		}
	case []interface{}:
		// Handle array results based on value extractor
		if cmd.ValueExtractor != "" && len(v) > 0 {
			// Simple array index extraction
			if idx, err := strconv.Atoi(cmd.ValueExtractor); err == nil && idx < len(v) {
				value = v[idx]
			}
		}
	default:
		return // Unsupported result type
	}
	
	// Extract labels
	attrs := make(map[string]string)
	for labelName, labelPath := range cmd.Labels {
		// Simple label extraction from result
		attrs[labelName] = fmt.Sprintf("%v", labelPath)
	}
	
	// Record metric based on type
	switch cmd.Type {
	case "gauge":
		mb.addGaugeMetric(metrics, cmd.Name, cmd.Description, "1", value, now, attrs)
	case "counter":
		mb.addSumMetric(metrics, cmd.Name, cmd.Description, "1", value, now, attrs)
	case "histogram":
		// For now, record as gauge
		mb.addGaugeMetric(metrics, cmd.Name, cmd.Description, "1", value, now, attrs)
	}
}

func (mb *metricBuilder) parseAndRecordSlaveInfo(metrics pmetric.MetricSlice, slaveID int, slaveInfo string, now pcommon.Timestamp) {
	// Parse slave info: ip=127.0.0.1,port=6380,state=online,offset=14,lag=0
	slaveParts := make(map[string]string)
	for _, part := range strings.Split(slaveInfo, ",") {
		kv := strings.Split(part, "=")
		if len(kv) == 2 {
			slaveParts[kv[0]] = kv[1]
		}
	}
	
	slaveAddr := fmt.Sprintf("%s:%s", slaveParts["ip"], slaveParts["port"])
	attrs := map[string]string{
		"slave_id": strconv.Itoa(slaveID),
		"address":  slaveAddr,
	}
	
	// Slave state
	online := 0
	if slaveParts["state"] == "online" {
		online = 1
	}
	mb.addGaugeMetric(metrics, "redis.replication.slave.online", "Slave online status",
		"bool", online, now, attrs)
	
	// Slave lag
	if lag, err := strconv.ParseInt(slaveParts["lag"], 10, 64); err == nil {
		mb.addGaugeMetric(metrics, "redis.replication.slave.lag", "Slave replication lag",
			"bytes", lag, now, attrs)
	}
	
	// Slave offset
	if offset, err := strconv.ParseInt(slaveParts["offset"], 10, 64); err == nil {
		mb.addGaugeMetric(metrics, "redis.replication.slave.offset", "Slave replication offset",
			"offset", offset, now, attrs)
	}
}

// Helper methods

func (mb *metricBuilder) addGaugeMetric(metrics pmetric.MetricSlice, name, description, unit string, value interface{}, ts pcommon.Timestamp, attributes map[string]string) {
	if value == nil {
		return
	}

	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit(unit)

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)

	// Set attributes
	if attributes != nil {
		for k, v := range attributes {
			dp.Attributes().PutStr(k, v)
		}
	}

	// Set value based on type
	switch v := value.(type) {
	case int:
		dp.SetIntValue(int64(v))
	case int32:
		dp.SetIntValue(int64(v))
	case int64:
		dp.SetIntValue(v)
	case float32:
		dp.SetDoubleValue(float64(v))
	case float64:
		dp.SetDoubleValue(v)
	}
}

func (mb *metricBuilder) addSumMetric(metrics pmetric.MetricSlice, name, description, unit string, value interface{}, ts pcommon.Timestamp, attributes map[string]string) {
	if value == nil {
		return
	}

	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit(unit)

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)

	// Set attributes
	if attributes != nil {
		for k, v := range attributes {
			dp.Attributes().PutStr(k, v)
		}
	}

	// Set value based on type
	switch v := value.(type) {
	case int:
		dp.SetIntValue(int64(v))
	case int32:
		dp.SetIntValue(int64(v))
	case int64:
		dp.SetIntValue(v)
	case float32:
		dp.SetDoubleValue(float64(v))
	case float64:
		dp.SetDoubleValue(v)
	}
}

func (mb *metricBuilder) addInfoMetric(metrics pmetric.MetricSlice, name, description string, ts pcommon.Timestamp, attributes map[string]string) {
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit("1")

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)
	dp.SetIntValue(1)

	// Set attributes
	if attributes != nil {
		for k, v := range attributes {
			dp.Attributes().PutStr(k, v)
		}
	}
}

// Utility functions

func calculateHitRatio(hits, misses string) float64 {
	h := parseFloatValue(hits)
	m := parseFloatValue(misses)
	total := h + m
	if total > 0 {
		return h / total
	}
	return 0
}

func parseArrayToMap(arr []interface{}) map[string]string {
	result := make(map[string]string)
	for i := 0; i < len(arr)-1; i += 2 {
		if key, ok := arr[i].(string); ok {
			if value, ok := arr[i+1].(string); ok {
				result[key] = value
			} else {
				result[key] = fmt.Sprintf("%v", arr[i+1])
			}
		}
	}
	return result
}

func mergeMaps(m1, m2 map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m1 {
		result[k] = v
	}
	for k, v := range m2 {
		result[k] = v
	}
	return result
}