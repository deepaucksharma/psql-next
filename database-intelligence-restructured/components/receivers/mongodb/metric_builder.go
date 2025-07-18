package mongodb

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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

func (mb *metricBuilder) recordServerStatusMetrics(metrics pmetric.MetricSlice, serverStatus bson.M, now pcommon.Timestamp) {
	// Connections
	if connections, ok := serverStatus["connections"].(bson.M); ok {
		mb.addGaugeMetric(metrics, "mongodb.connections.current", "Current number of connections",
			"connections", connections["current"], now, nil)
		mb.addGaugeMetric(metrics, "mongodb.connections.available", "Number of available connections",
			"connections", connections["available"], now, nil)
		mb.addSumMetric(metrics, "mongodb.connections.total_created", "Total connections created",
			"connections", connections["totalCreated"], now, nil)
	}

	// Memory
	if mem, ok := serverStatus["mem"].(bson.M); ok {
		mb.addGaugeMetric(metrics, "mongodb.memory.resident", "Resident memory usage",
			"MiB", mem["resident"], now, nil)
		mb.addGaugeMetric(metrics, "mongodb.memory.virtual", "Virtual memory usage",
			"MiB", mem["virtual"], now, nil)
	}

	// Operations
	if opcounters, ok := serverStatus["opcounters"].(bson.M); ok {
		mb.addSumMetric(metrics, "mongodb.operations.count", "Number of operations",
			"operations", opcounters["insert"], now, map[string]string{"type": "insert"})
		mb.addSumMetric(metrics, "mongodb.operations.count", "Number of operations",
			"operations", opcounters["query"], now, map[string]string{"type": "query"})
		mb.addSumMetric(metrics, "mongodb.operations.count", "Number of operations",
			"operations", opcounters["update"], now, map[string]string{"type": "update"})
		mb.addSumMetric(metrics, "mongodb.operations.count", "Number of operations",
			"operations", opcounters["delete"], now, map[string]string{"type": "delete"})
		mb.addSumMetric(metrics, "mongodb.operations.count", "Number of operations",
			"operations", opcounters["getmore"], now, map[string]string{"type": "getmore"})
		mb.addSumMetric(metrics, "mongodb.operations.count", "Number of operations",
			"operations", opcounters["command"], now, map[string]string{"type": "command"})
	}

	// Network
	if network, ok := serverStatus["network"].(bson.M); ok {
		mb.addSumMetric(metrics, "mongodb.network.bytes_in", "Network bytes received",
			"By", network["bytesIn"], now, nil)
		mb.addSumMetric(metrics, "mongodb.network.bytes_out", "Network bytes sent",
			"By", network["bytesOut"], now, nil)
		mb.addSumMetric(metrics, "mongodb.network.requests", "Number of network requests",
			"requests", network["numRequests"], now, nil)
	}

	// Locks
	if locks, ok := serverStatus["locks"].(bson.M); ok {
		for lockType, lockData := range locks {
			if lockInfo, ok := lockData.(bson.M); ok {
				if acquireCount, ok := lockInfo["acquireCount"].(bson.M); ok {
					for mode, count := range acquireCount {
						mb.addSumMetric(metrics, "mongodb.locks.acquire.count", "Lock acquire count",
							"locks", count, now, map[string]string{"type": lockType, "mode": mode})
					}
				}
				if timeAcquiringMicros, ok := lockInfo["timeAcquiringMicros"].(bson.M); ok {
					for mode, micros := range timeAcquiringMicros {
						mb.addSumMetric(metrics, "mongodb.locks.acquire.time", "Time acquiring locks",
							"us", micros, now, map[string]string{"type": lockType, "mode": mode})
					}
				}
			}
		}
	}

	// WiredTiger metrics
	if mb.config.Metrics.WiredTiger {
		if wiredTiger, ok := serverStatus["wiredTiger"].(bson.M); ok {
			mb.recordWiredTigerMetrics(metrics, wiredTiger, now)
		}
	}
}

func (mb *metricBuilder) recordDatabaseStats(metrics pmetric.MetricSlice, dbName string, stats bson.M, now pcommon.Timestamp) {
	attrs := map[string]string{"database": dbName}

	mb.addGaugeMetric(metrics, "mongodb.database.size", "Database size on disk",
		"By", stats["dataSize"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.database.storage_size", "Database storage size",
		"By", stats["storageSize"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.database.index_size", "Database index size",
		"By", stats["indexSize"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.database.collections", "Number of collections",
		"collections", stats["collections"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.database.objects", "Number of objects",
		"objects", stats["objects"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.database.indexes", "Number of indexes",
		"indexes", stats["indexes"], now, attrs)
}

func (mb *metricBuilder) recordCollectionStats(metrics pmetric.MetricSlice, dbName, collName string, stats bson.M, now pcommon.Timestamp) {
	attrs := map[string]string{
		"database":   dbName,
		"collection": collName,
	}

	mb.addGaugeMetric(metrics, "mongodb.collection.size", "Collection size",
		"By", stats["size"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.collection.storage_size", "Collection storage size",
		"By", stats["storageSize"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.collection.count", "Number of documents",
		"documents", stats["count"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.collection.avg_obj_size", "Average object size",
		"By", stats["avgObjSize"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.collection.indexes", "Number of indexes",
		"indexes", stats["nindexes"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.collection.index_size", "Total index size",
		"By", stats["totalIndexSize"], now, attrs)

	// Index sizes
	if indexSizes, ok := stats["indexSizes"].(bson.M); ok {
		for indexName, size := range indexSizes {
			indexAttrs := map[string]string{
				"database":   dbName,
				"collection": collName,
				"index":      indexName,
			}
			mb.addGaugeMetric(metrics, "mongodb.index.size", "Index size",
				"By", size, now, indexAttrs)
		}
	}
}

func (mb *metricBuilder) recordIndexStats(metrics pmetric.MetricSlice, dbName, collName string, indexStats []bson.M, now pcommon.Timestamp) {
	for _, idxStat := range indexStats {
		indexName, _ := idxStat["name"].(string)
		attrs := map[string]string{
			"database":   dbName,
			"collection": collName,
			"index":      indexName,
		}

		if accesses, ok := idxStat["accesses"].(bson.M); ok {
			mb.addSumMetric(metrics, "mongodb.index.access.count", "Index access count",
				"accesses", accesses["ops"], now, attrs)
		}
	}
}

func (mb *metricBuilder) recordCurrentOp(metrics pmetric.MetricSlice, currentOp bson.M, now pcommon.Timestamp) {
	if inprog, ok := currentOp["inprog"].(bson.A); ok {
		opCounts := make(map[string]int)
		opDurations := make(map[string][]int64)

		for _, op := range inprog {
			if opDoc, ok := op.(bson.M); ok {
				opType, _ := opDoc["op"].(string)
				if opType == "" {
					opType = "unknown"
				}
				opCounts[opType]++

				// Track operation duration
				if microsecs, ok := opDoc["microsecs_running"].(int64); ok {
					opDurations[opType] = append(opDurations[opType], microsecs)
				}
			}
		}

		// Record operation counts
		for opType, count := range opCounts {
			mb.addGaugeMetric(metrics, "mongodb.operations.active", "Number of active operations",
				"operations", count, now, map[string]string{"type": opType})
		}

		// Record operation durations (max)
		for opType, durations := range opDurations {
			maxDuration := int64(0)
			for _, d := range durations {
				if d > maxDuration {
					maxDuration = d
				}
			}
			mb.addGaugeMetric(metrics, "mongodb.operations.duration.max", "Maximum operation duration",
				"us", maxDuration, now, map[string]string{"type": opType})
		}
	}
}

func (mb *metricBuilder) recordReplicaSetStatus(metrics pmetric.MetricSlice, rsStatus bson.M, now pcommon.Timestamp) {
	setName, _ := rsStatus["set"].(string)
	myState, _ := rsStatus["myState"].(int32)

	attrs := map[string]string{"replica_set": setName}
	mb.addGaugeMetric(metrics, "mongodb.replica_set.state", "Replica set member state",
		"state", myState, now, attrs)

	// Record member states
	if members, ok := rsStatus["members"].(bson.A); ok {
		for _, member := range members {
			if m, ok := member.(bson.M); ok {
				name, _ := m["name"].(string)
				state, _ := m["state"].(int32)
				stateStr, _ := m["stateStr"].(string)
				health, _ := m["health"].(float64)

				memberAttrs := map[string]string{
					"replica_set": setName,
					"member":      name,
					"state":       stateStr,
				}

				mb.addGaugeMetric(metrics, "mongodb.replica_set.member.state", "Member state",
					"state", state, now, memberAttrs)
				mb.addGaugeMetric(metrics, "mongodb.replica_set.member.health", "Member health",
					"health", health, now, memberAttrs)

				// Ping time for remote members
				if pingMs, ok := m["pingMs"].(int64); ok {
					mb.addGaugeMetric(metrics, "mongodb.replica_set.member.ping", "Ping time to member",
						"ms", pingMs, now, memberAttrs)
				}
			}
		}
	}
}

func (mb *metricBuilder) recordOplogMetrics(metrics pmetric.MetricSlice, collStats, firstDoc, lastDoc bson.M, now pcommon.Timestamp) {
	attrs := map[string]string{"collection": "oplog.rs"}

	mb.addGaugeMetric(metrics, "mongodb.oplog.size", "Oplog size",
		"By", collStats["size"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.oplog.storage_size", "Oplog storage size",
		"By", collStats["storageSize"], now, attrs)
	mb.addGaugeMetric(metrics, "mongodb.oplog.count", "Number of oplog entries",
		"entries", collStats["count"], now, attrs)

	// Calculate oplog window
	if firstTs, ok := firstDoc["ts"].(bson.Timestamp); ok {
		if lastTs, ok := lastDoc["ts"].(bson.Timestamp); ok {
			windowSecs := lastTs.T - firstTs.T
			mb.addGaugeMetric(metrics, "mongodb.oplog.window", "Oplog window",
				"s", windowSecs, now, attrs)
		}
	}
}

func (mb *metricBuilder) recordReplicationLag(metrics pmetric.MetricSlice, memberLags map[string]time.Duration, now pcommon.Timestamp) {
	for member, lag := range memberLags {
		attrs := map[string]string{"member": member}
		mb.addGaugeMetric(metrics, "mongodb.replica_set.lag", "Replication lag",
			"s", lag.Seconds(), now, attrs)

		// Check if lag exceeds threshold
		if lag > mb.config.ReplicaSet.LagThreshold {
			mb.addGaugeMetric(metrics, "mongodb.replica_set.lag.threshold_exceeded", "Lag threshold exceeded",
				"bool", 1, now, attrs)
		} else {
			mb.addGaugeMetric(metrics, "mongodb.replica_set.lag.threshold_exceeded", "Lag threshold exceeded",
				"bool", 0, now, attrs)
		}
	}
}

func (mb *metricBuilder) recordWiredTigerMetrics(metrics pmetric.MetricSlice, wiredTiger bson.M, now pcommon.Timestamp) {
	// Cache metrics
	if cache, ok := wiredTiger["cache"].(bson.M); ok {
		mb.addGaugeMetric(metrics, "mongodb.wiredtiger.cache.bytes_in_cache", "Bytes in cache",
			"By", cache["bytes currently in the cache"], now, nil)
		mb.addGaugeMetric(metrics, "mongodb.wiredtiger.cache.max_bytes", "Maximum cache size",
			"By", cache["maximum bytes configured"], now, nil)
		mb.addSumMetric(metrics, "mongodb.wiredtiger.cache.bytes_read", "Bytes read into cache",
			"By", cache["bytes read into cache"], now, nil)
		mb.addSumMetric(metrics, "mongodb.wiredtiger.cache.bytes_written", "Bytes written from cache",
			"By", cache["bytes written from cache"], now, nil)
		mb.addSumMetric(metrics, "mongodb.wiredtiger.cache.evictions", "Pages evicted",
			"pages", cache["pages evicted by application threads"], now, nil)
	}

	// Transaction metrics
	if transaction, ok := wiredTiger["transaction"].(bson.M); ok {
		mb.addSumMetric(metrics, "mongodb.wiredtiger.transactions.begins", "Transactions started",
			"transactions", transaction["transaction begins"], now, nil)
		mb.addSumMetric(metrics, "mongodb.wiredtiger.transactions.commits", "Transactions committed",
			"transactions", transaction["transactions committed"], now, nil)
		mb.addSumMetric(metrics, "mongodb.wiredtiger.transactions.rollbacks", "Transactions rolled back",
			"transactions", transaction["transactions rolled back"], now, nil)
	}
}

func (mb *metricBuilder) recordBalancerMetrics(metrics pmetric.MetricSlice, isEnabled bool, activeMigrations int64, now pcommon.Timestamp) {
	enabledVal := 0
	if isEnabled {
		enabledVal = 1
	}

	mb.addGaugeMetric(metrics, "mongodb.balancer.enabled", "Balancer enabled status",
		"bool", enabledVal, now, nil)
	mb.addGaugeMetric(metrics, "mongodb.balancer.migrations.active", "Active chunk migrations",
		"migrations", activeMigrations, now, nil)
}

func (mb *metricBuilder) recordChunkDistribution(metrics pmetric.MetricSlice, chunkDist []bson.M, now pcommon.Timestamp) {
	for _, dist := range chunkDist {
		shard, _ := dist["_id"].(string)
		count, _ := dist["count"].(int32)
		
		attrs := map[string]string{"shard": shard}
		mb.addGaugeMetric(metrics, "mongodb.chunks.count", "Number of chunks on shard",
			"chunks", count, now, attrs)
	}
}

func (mb *metricBuilder) recordCustomMetric(metrics pmetric.MetricSlice, config CustomMetricConfig, result bson.M, now pcommon.Timestamp) {
	// Extract value using path
	value := mb.extractValueFromPath(result, config.ValuePath)
	if value == nil {
		return
	}

	// Extract labels
	attrs := make(map[string]string)
	for labelName, labelPath := range config.Labels {
		if labelValue := mb.extractValueFromPath(result, labelPath); labelValue != nil {
			attrs[labelName] = fmt.Sprintf("%v", labelValue)
		}
	}

	// Add metric based on type
	switch config.Type {
	case "gauge":
		mb.addGaugeMetric(metrics, config.Name, config.Description,
			"1", value, now, attrs)
	case "counter":
		mb.addSumMetric(metrics, config.Name, config.Description,
			"1", value, now, attrs)
	case "histogram":
		// For histogram, we'd need more configuration
		// For now, just record as gauge
		mb.addGaugeMetric(metrics, config.Name, config.Description,
			"1", value, now, attrs)
	}
}

func (mb *metricBuilder) extractValueFromPath(data bson.M, path string) interface{} {
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		switch v := current.(type) {
		case bson.M:
			current = v[part]
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

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