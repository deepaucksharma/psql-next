package mongodb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type scraper struct {
	logger   *zap.Logger
	config   *Config
	settings receiver.CreateSettings
	client   *mongo.Client
	mb       *metricBuilder
}

func newScraper(settings receiver.CreateSettings, cfg *Config) *scraper {
	return &scraper{
		logger:   settings.Logger,
		config:   cfg,
		settings: settings,
		mb:       newMetricBuilder(cfg),
	}
}

func (s *scraper) start(ctx context.Context, host component.Host) error {
	clientOpts := options.Client().ApplyURI(s.config.URI)

	// Configure connection pool
	if s.config.MaxPoolSize > 0 {
		clientOpts.SetMaxPoolSize(s.config.MaxPoolSize)
	}
	if s.config.MinPoolSize > 0 {
		clientOpts.SetMinPoolSize(s.config.MinPoolSize)
	}
	if s.config.ConnectTimeout > 0 {
		clientOpts.SetConnectTimeout(s.config.ConnectTimeout)
	}
	if s.config.SocketTimeout > 0 {
		clientOpts.SetSocketTimeout(s.config.SocketTimeout)
	}

	// Configure TLS if enabled
	if s.config.TLS.Enabled {
		tlsConfig, err := s.createTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
		clientOpts.SetTLSConfig(tlsConfig)
	}

	// Create client
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		client.Disconnect(ctx)
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	s.client = client
	s.logger.Info("Connected to MongoDB",
		zap.String("uri", s.config.getURIMasked()),
		zap.String("host", s.config.extractHost()))

	return nil
}

func (s *scraper) shutdown(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
	}
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics().AppendEmpty()
	rs := rms.Resource()

	// Set resource attributes
	rs.Attributes().PutStr("mongodb.instance", s.config.extractHost())
	if s.config.Database != "" {
		rs.Attributes().PutStr("mongodb.database", s.config.Database)
	}
	for k, v := range s.config.ResourceAttributes {
		rs.Attributes().PutStr(k, v)
	}

	ilm := rms.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("otelcol/mongodb")

	now := pcommon.NewTimestampFromTime(time.Now())

	// Collect server status metrics
	if s.config.Metrics.ServerStatus {
		if err := s.collectServerStatus(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect server status", zap.Error(err))
		}
	}

	// Collect database stats
	if s.config.Metrics.DatabaseStats {
		if err := s.collectDatabaseStats(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect database stats", zap.Error(err))
		}
	}

	// Collect collection stats
	if s.config.Metrics.CollectionStats {
		if err := s.collectCollectionStats(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect collection stats", zap.Error(err))
		}
	}

	// Collect index stats
	if s.config.Metrics.IndexStats {
		if err := s.collectIndexStats(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect index stats", zap.Error(err))
		}
	}

	// Collect current operations
	if s.config.Metrics.CurrentOp {
		if err := s.collectCurrentOp(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect current operations", zap.Error(err))
		}
	}

	// Collect replica set metrics
	if s.config.ReplicaSet.Enabled {
		if err := s.collectReplicaSetMetrics(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect replica set metrics", zap.Error(err))
		}
	}

	// Collect sharding metrics
	if s.config.Sharding.Enabled {
		if err := s.collectShardingMetrics(ctx, ilm.Metrics(), now); err != nil {
			s.logger.Error("Failed to collect sharding metrics", zap.Error(err))
		}
	}

	// Collect custom metrics
	for _, metric := range s.config.Metrics.CustomMetrics {
		if err := s.collectCustomMetric(ctx, ilm.Metrics(), now, metric); err != nil {
			s.logger.Error("Failed to collect custom metric",
				zap.String("metric", metric.Name),
				zap.Error(err))
		}
	}

	return md, nil
}

func (s *scraper) collectServerStatus(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	var result bson.M
	err := s.client.Database("admin").RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&result)
	if err != nil {
		return fmt.Errorf("serverStatus command failed: %w", err)
	}

	// Extract and record metrics
	s.mb.recordServerStatusMetrics(metrics, result, now)
	return nil
}

func (s *scraper) collectDatabaseStats(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	databases := []string{}
	
	if s.config.Database != "" {
		databases = append(databases, s.config.Database)
	} else {
		// List all databases
		dbs, err := s.client.ListDatabaseNames(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("failed to list databases: %w", err)
		}
		databases = dbs
	}

	for _, dbName := range databases {
		var result bson.M
		err := s.client.Database(dbName).RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&result)
		if err != nil {
			s.logger.Warn("Failed to get stats for database",
				zap.String("database", dbName),
				zap.Error(err))
			continue
		}

		s.mb.recordDatabaseStats(metrics, dbName, result, now)
	}

	return nil
}

func (s *scraper) collectCollectionStats(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	databases := []string{}
	
	if s.config.Database != "" {
		databases = append(databases, s.config.Database)
	} else {
		dbs, err := s.client.ListDatabaseNames(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("failed to list databases: %w", err)
		}
		databases = dbs
	}

	for _, dbName := range databases {
		db := s.client.Database(dbName)
		
		collections := []string{}
		if len(s.config.Collections) > 0 {
			collections = s.config.Collections
		} else {
			colls, err := db.ListCollectionNames(ctx, bson.M{})
			if err != nil {
				s.logger.Warn("Failed to list collections",
					zap.String("database", dbName),
					zap.Error(err))
				continue
			}
			collections = colls
		}

		for _, collName := range collections {
			var result bson.M
			err := db.RunCommand(ctx, bson.D{
				{Key: "collStats", Value: collName},
			}).Decode(&result)
			if err != nil {
				s.logger.Warn("Failed to get stats for collection",
					zap.String("database", dbName),
					zap.String("collection", collName),
					zap.Error(err))
				continue
			}

			s.mb.recordCollectionStats(metrics, dbName, collName, result, now)
		}
	}

	return nil
}

func (s *scraper) collectIndexStats(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	databases := []string{}
	
	if s.config.Database != "" {
		databases = append(databases, s.config.Database)
	} else {
		dbs, err := s.client.ListDatabaseNames(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("failed to list databases: %w", err)
		}
		databases = dbs
	}

	for _, dbName := range databases {
		db := s.client.Database(dbName)
		
		collections := []string{}
		if len(s.config.Collections) > 0 {
			collections = s.config.Collections
		} else {
			colls, err := db.ListCollectionNames(ctx, bson.M{})
			if err != nil {
				s.logger.Warn("Failed to list collections",
					zap.String("database", dbName),
					zap.Error(err))
				continue
			}
			collections = colls
		}

		for _, collName := range collections {
			// Get index stats using aggregate
			cursor, err := db.Collection(collName).Aggregate(ctx, bson.A{
				bson.M{"$indexStats": bson.M{}},
			})
			if err != nil {
				s.logger.Warn("Failed to get index stats",
					zap.String("database", dbName),
					zap.String("collection", collName),
					zap.Error(err))
				continue
			}
			defer cursor.Close(ctx)

			var indexStats []bson.M
			if err := cursor.All(ctx, &indexStats); err != nil {
				s.logger.Warn("Failed to decode index stats",
					zap.String("database", dbName),
					zap.String("collection", collName),
					zap.Error(err))
				continue
			}

			s.mb.recordIndexStats(metrics, dbName, collName, indexStats, now)
		}
	}

	return nil
}

func (s *scraper) collectCurrentOp(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	var result bson.M
	err := s.client.Database("admin").RunCommand(ctx, bson.D{
		{Key: "currentOp", Value: 1},
		{Key: "allUsers", Value: true},
	}).Decode(&result)
	if err != nil {
		return fmt.Errorf("currentOp command failed: %w", err)
	}

	s.mb.recordCurrentOp(metrics, result, now)
	return nil
}

func (s *scraper) collectReplicaSetMetrics(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Get replica set status
	var rsStatus bson.M
	err := s.client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}}).Decode(&rsStatus)
	if err != nil {
		// Not a replica set member
		return nil
	}

	s.mb.recordReplicaSetStatus(metrics, rsStatus, now)

	// Collect oplog metrics if enabled
	if s.config.ReplicaSet.CollectOplogMetrics {
		if err := s.collectOplogMetrics(ctx, metrics, now); err != nil {
			s.logger.Warn("Failed to collect oplog metrics", zap.Error(err))
		}
	}

	// Collect replication lag metrics if enabled
	if s.config.ReplicaSet.CollectReplLagMetrics {
		if err := s.collectReplLagMetrics(ctx, metrics, rsStatus, now); err != nil {
			s.logger.Warn("Failed to collect replication lag metrics", zap.Error(err))
		}
	}

	return nil
}

func (s *scraper) collectOplogMetrics(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	oplogColl := s.client.Database("local").Collection("oplog.rs")

	// Get oplog size
	var collStats bson.M
	err := s.client.Database("local").RunCommand(ctx, bson.D{
		{Key: "collStats", Value: "oplog.rs"},
	}).Decode(&collStats)
	if err != nil {
		return fmt.Errorf("failed to get oplog stats: %w", err)
	}

	// Get first and last oplog entries
	firstOpts := options.FindOne().SetSort(bson.D{{Key: "ts", Value: 1}})
	lastOpts := options.FindOne().SetSort(bson.D{{Key: "ts", Value: -1}})

	var firstDoc, lastDoc bson.M
	if err := oplogColl.FindOne(ctx, bson.M{}, firstOpts).Decode(&firstDoc); err != nil {
		return fmt.Errorf("failed to get first oplog entry: %w", err)
	}
	if err := oplogColl.FindOne(ctx, bson.M{}, lastOpts).Decode(&lastDoc); err != nil {
		return fmt.Errorf("failed to get last oplog entry: %w", err)
	}

	s.mb.recordOplogMetrics(metrics, collStats, firstDoc, lastDoc, now)
	return nil
}

func (s *scraper) collectReplLagMetrics(ctx context.Context, metrics pmetric.MetricSlice, rsStatus bson.M, now pcommon.Timestamp) error {
	members, ok := rsStatus["members"].(bson.A)
	if !ok {
		return fmt.Errorf("invalid replica set status format")
	}

	var primaryOptime time.Time
	memberLags := make(map[string]time.Duration)

	// Find primary optime
	for _, member := range members {
		m, ok := member.(bson.M)
		if !ok {
			continue
		}

		stateStr, _ := m["stateStr"].(string)
		if stateStr == "PRIMARY" {
			if optime, ok := m["optime"].(bson.M); ok {
				if ts, ok := optime["ts"].(bson.Timestamp); ok {
					primaryOptime = time.Unix(int64(ts.T), 0)
				}
			}
		}
	}

	// Calculate lag for each secondary
	for _, member := range members {
		m, ok := member.(bson.M)
		if !ok {
			continue
		}

		name, _ := m["name"].(string)
		stateStr, _ := m["stateStr"].(string)

		if stateStr == "SECONDARY" || stateStr == "ARBITER" {
			if optime, ok := m["optime"].(bson.M); ok {
				if ts, ok := optime["ts"].(bson.Timestamp); ok {
					memberOptime := time.Unix(int64(ts.T), 0)
					lag := primaryOptime.Sub(memberOptime)
					memberLags[name] = lag
				}
			}
		}
	}

	s.mb.recordReplicationLag(metrics, memberLags, now)
	return nil
}

func (s *scraper) collectShardingMetrics(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	// Check if this is a mongos or config server
	var isMaster bson.M
	err := s.client.Database("admin").RunCommand(ctx, bson.D{{Key: "isMaster", Value: 1}}).Decode(&isMaster)
	if err != nil {
		return fmt.Errorf("isMaster command failed: %w", err)
	}

	msg, _ := isMaster["msg"].(string)
	if msg != "isdbgrid" {
		// Not a mongos, skip sharding metrics
		return nil
	}

	// Collect balancer status
	if s.config.Sharding.CollectBalancerMetrics {
		if err := s.collectBalancerMetrics(ctx, metrics, now); err != nil {
			s.logger.Warn("Failed to collect balancer metrics", zap.Error(err))
		}
	}

	// Collect chunk distribution
	if s.config.Sharding.CollectChunkMetrics {
		if err := s.collectChunkMetrics(ctx, metrics, now); err != nil {
			s.logger.Warn("Failed to collect chunk metrics", zap.Error(err))
		}
	}

	return nil
}

func (s *scraper) collectBalancerMetrics(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	configDB := s.client.Database("config")

	// Get balancer status
	var balancerDoc bson.M
	err := configDB.Collection("settings").FindOne(ctx, bson.M{"_id": "balancer"}).Decode(&balancerDoc)
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("failed to get balancer status: %w", err)
	}

	isEnabled := true
	if balancerDoc != nil {
		if stopped, ok := balancerDoc["stopped"].(bool); ok {
			isEnabled = !stopped
		}
	}

	// Get active migrations
	activeMigrations, err := configDB.Collection("migrations").CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to count active migrations: %w", err)
	}

	s.mb.recordBalancerMetrics(metrics, isEnabled, activeMigrations, now)
	return nil
}

func (s *scraper) collectChunkMetrics(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp) error {
	configDB := s.client.Database("config")

	// Aggregate chunks by shard
	cursor, err := configDB.Collection("chunks").Aggregate(ctx, bson.A{
		bson.M{"$group": bson.M{
			"_id":   "$shard",
			"count": bson.M{"$sum": 1},
		}},
	})
	if err != nil {
		return fmt.Errorf("failed to aggregate chunks: %w", err)
	}
	defer cursor.Close(ctx)

	var chunkDist []bson.M
	if err := cursor.All(ctx, &chunkDist); err != nil {
		return fmt.Errorf("failed to decode chunk distribution: %w", err)
	}

	s.mb.recordChunkDistribution(metrics, chunkDist, now)
	return nil
}

func (s *scraper) collectCustomMetric(ctx context.Context, metrics pmetric.MetricSlice, now pcommon.Timestamp, config CustomMetricConfig) error {
	db := s.client.Database(config.Database)
	if db == nil {
		db = s.client.Database("admin")
	}

	var result bson.M
	err := db.RunCommand(ctx, bson.D{{Key: config.Command, Value: config.Collection}}).Decode(&result)
	if err != nil {
		return fmt.Errorf("custom command %s failed: %w", config.Command, err)
	}

	s.mb.recordCustomMetric(metrics, config, result, now)
	return nil
}

func (s *scraper) createTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: s.config.TLS.Insecure,
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