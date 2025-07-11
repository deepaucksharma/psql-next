package adaptivesampler

import (
    "context"
    "fmt"
    "math"
	"strconv"
	"strings"
    "sync"
    "time"
    
    "github.com/go-redis/redis/v8"
    "go.uber.org/zap"
)

// AdaptiveAlgorithm implements workload-based sampling
type AdaptiveAlgorithm struct {
    logger      *zap.Logger
    redisClient *redis.Client
    config      *Config
    
    // Local cache
    mu           sync.RWMutex
    sampleRates  map[string]float64
    queryStats   map[string]*QueryStats
    lastSync     time.Time
}

// QueryStats tracks query performance statistics
type QueryStats struct {
    QueryID          string
    ExecutionCount   int64
    TotalDuration    float64
    MeanDuration     float64
    StdDevDuration   float64
    ErrorRate        float64
    LastSeen         time.Time
    ImportanceScore  float64
}

// NewAdaptiveAlgorithm creates a new adaptive sampling algorithm
func NewAdaptiveAlgorithm(logger *zap.Logger, redisClient *redis.Client, config *Config) *AdaptiveAlgorithm {
    return &AdaptiveAlgorithm{
        logger:      logger,
        redisClient: redisClient,
        config:      config,
        sampleRates: make(map[string]float64),
        queryStats:  make(map[string]*QueryStats),
    }
}

// CalculateSampleRate determines the sampling rate for a query
func (aa *AdaptiveAlgorithm) CalculateSampleRate(ctx context.Context, queryID string, metrics QueryMetrics) float64 {
    aa.mu.Lock()
    defer aa.mu.Unlock()
    
    // Update query statistics
    stats := aa.updateQueryStats(queryID, metrics)
    
    // Calculate importance score
    importanceScore := aa.calculateImportanceScore(stats, metrics)
    stats.ImportanceScore = importanceScore
    
    // Determine sample rate based on importance
    sampleRate := aa.determineSampleRate(importanceScore, metrics)
    
    // Apply rate limiting
    sampleRate = aa.applyRateLimiting(queryID, sampleRate)
    
    // Store for future reference
    aa.sampleRates[queryID] = sampleRate
    
    // Sync with Redis periodically
    if time.Since(aa.lastSync) > aa.config.SyncInterval {
        go aa.syncWithRedis(ctx)
    }
    
    return sampleRate
}

// updateQueryStats updates the statistics for a query
func (aa *AdaptiveAlgorithm) updateQueryStats(queryID string, metrics QueryMetrics) *QueryStats {
    stats, exists := aa.queryStats[queryID]
    if !exists {
        stats = &QueryStats{
            QueryID: queryID,
        }
        aa.queryStats[queryID] = stats
    }
    
    // Update statistics
    stats.ExecutionCount++
    stats.TotalDuration += metrics.Duration
    stats.MeanDuration = stats.TotalDuration / float64(stats.ExecutionCount)
    
    // Update standard deviation
    if stats.ExecutionCount > 1 {
        variance := math.Pow(metrics.Duration-stats.MeanDuration, 2) / float64(stats.ExecutionCount-1)
        stats.StdDevDuration = math.Sqrt(variance)
    }
    
    // Update error rate
    if metrics.HasError {
        stats.ErrorRate = (stats.ErrorRate*float64(stats.ExecutionCount-1) + 1) / float64(stats.ExecutionCount)
    } else {
        stats.ErrorRate = stats.ErrorRate * float64(stats.ExecutionCount-1) / float64(stats.ExecutionCount)
    }
    
    stats.LastSeen = time.Now()
    
    return stats
}

// calculateImportanceScore calculates how important a query is for monitoring
func (aa *AdaptiveAlgorithm) calculateImportanceScore(stats *QueryStats, metrics QueryMetrics) float64 {
    score := 0.0
    
    // Factor 1: Query cost (40% weight)
    costScore := math.Min(metrics.Duration/aa.config.HighCostThreshold, 1.0) * 0.4
    score += costScore
    
    // Factor 2: Error rate (30% weight)
    errorScore := stats.ErrorRate * 0.3
    score += errorScore
    
    // Factor 3: Variability (20% weight)
    variabilityScore := 0.0
    if stats.MeanDuration > 0 {
        cv := stats.StdDevDuration / stats.MeanDuration // Coefficient of variation
        variabilityScore = math.Min(cv, 1.0) * 0.2
    }
    score += variabilityScore
    
    // Factor 4: Business criticality (10% weight)
    criticalityScore := 0.0
    if metrics.IsCritical {
        criticalityScore = 0.1
    }
    score += criticalityScore
    
    return score
}

// determineSampleRate converts importance score to sample rate
func (aa *AdaptiveAlgorithm) determineSampleRate(importanceScore float64, metrics QueryMetrics) float64 {
    // Always sample errors
    if metrics.HasError {
        return 1.0
    }
    
    // High importance queries (score > 0.7)
    if importanceScore > 0.7 {
        return 1.0
    }
    
    // Medium importance (score 0.3-0.7)
    if importanceScore > 0.3 {
        return 0.5 + (importanceScore-0.3)*1.25 // 0.5 to 1.0
    }
    
    // Low importance (score < 0.3)
    return math.Max(aa.config.MinSampleRate, importanceScore*1.67) // 0.1 to 0.5
}

// applyRateLimiting ensures we don't exceed sampling limits
func (aa *AdaptiveAlgorithm) applyRateLimiting(queryID string, proposedRate float64) float64 {
    // Get current sampling volume
    currentVolume := aa.getCurrentSamplingVolume()
    
    // If we're under the limit, use proposed rate
    if currentVolume < float64(aa.config.MaxRecordsPerSecond)*0.8 {
        return proposedRate
    }
    
    // If we're near the limit, reduce sampling
    reductionFactor := float64(aa.config.MaxRecordsPerSecond) / currentVolume
    return proposedRate * reductionFactor * 0.8 // 80% to leave headroom
}

// getCurrentSamplingVolume estimates current sampling rate across all queries
func (aa *AdaptiveAlgorithm) getCurrentSamplingVolume() float64 {
    totalVolume := 0.0
    for queryID, stats := range aa.queryStats {
        rate := aa.sampleRates[queryID]
        qps := float64(stats.ExecutionCount) / time.Since(stats.LastSeen).Seconds()
        totalVolume += qps * rate
    }
    return totalVolume
}

// syncWithRedis synchronizes local state with Redis
func (aa *AdaptiveAlgorithm) syncWithRedis(ctx context.Context) {
    aa.lastSync = time.Now()
    
    // Push local stats to Redis
    pipe := aa.redisClient.Pipeline()
    
    for queryID, stats := range aa.queryStats {
        key := fmt.Sprintf("adaptive:stats:%s", queryID)
        data := map[string]interface{}{
            "execution_count":  stats.ExecutionCount,
            "mean_duration":    stats.MeanDuration,
            "error_rate":       stats.ErrorRate,
            "importance_score": stats.ImportanceScore,
            "sample_rate":      aa.sampleRates[queryID],
        }
        
        pipe.HMSet(ctx, key, data)
        pipe.Expire(ctx, key, 24*time.Hour)
    }
    
    if _, err := pipe.Exec(ctx); err != nil {
        aa.logger.Warn("Failed to sync with Redis", zap.Error(err))
    }
}

// LoadFromRedis loads sampling state from Redis
func (aa *AdaptiveAlgorithm) LoadFromRedis(ctx context.Context) error {
    // Scan for all adaptive stats keys
    var cursor uint64
    for {
        keys, nextCursor, err := aa.redisClient.Scan(ctx, cursor, "adaptive:stats:*", 100).Result()
        if err != nil {
            return err
        }
        
        for _, key := range keys {
            queryID := strings.TrimPrefix(key, "adaptive:stats:")
            data, err := aa.redisClient.HGetAll(ctx, key).Result()
            if err != nil {
                continue
            }
            
            // Reconstruct stats
            stats := &QueryStats{
                QueryID: queryID,
            }
            
            if v, ok := data["execution_count"]; ok {
                if count, err := strconv.ParseInt(v, 10, 64); err == nil {
                    stats.ExecutionCount = count
                } else {
                    aa.logger.Warn("Failed to parse execution_count", zap.String("value", v), zap.Error(err))
                }
            }
            if v, ok := data["mean_duration"]; ok {
                if duration, err := strconv.ParseFloat(v, 64); err == nil {
                    stats.MeanDuration = duration
                } else {
                    aa.logger.Warn("Failed to parse mean_duration", zap.String("value", v), zap.Error(err))
                }
            }
            if v, ok := data["error_rate"]; ok {
                if rate, err := strconv.ParseFloat(v, 64); err == nil {
                    stats.ErrorRate = rate
                } else {
                    aa.logger.Warn("Failed to parse error_rate", zap.String("value", v), zap.Error(err))
                }
            }
            if v, ok := data["importance_score"]; ok {
                if score, err := strconv.ParseFloat(v, 64); err == nil {
                    stats.ImportanceScore = score
                } else {
                    aa.logger.Warn("Failed to parse importance_score", zap.String("value", v), zap.Error(err))
                }
            }
            
            aa.queryStats[queryID] = stats
            
            if v, ok := data["sample_rate"]; ok {
                if rate, err := strconv.ParseFloat(v, 64); err == nil {
                    aa.sampleRates[queryID] = rate
                } else {
                    aa.logger.Warn("Failed to parse sample_rate", zap.String("value", v), zap.Error(err))
                }
            }
        }
        
        cursor = nextCursor
        if cursor == 0 {
            break
        }
    }
    
    return nil
}

// QueryMetrics represents metrics for a single query execution
type QueryMetrics struct {
    Duration   float64
    HasError   bool
    IsCritical bool
    RowCount   int64
    DatabaseName string
}