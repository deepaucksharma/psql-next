package planattributeextractor

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)



// planAttributeExtractor is the processor implementation
type planAttributeExtractor struct {
	config   *Config
	logger   *zap.Logger
	consumer consumer.Logs
}

// newPlanAttributeExtractor creates a new plan attribute extractor processor
func newPlanAttributeExtractor(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *planAttributeExtractor {
	return &planAttributeExtractor{
		config:   cfg,
		logger:   logger,
		consumer: consumer,
	}
}

// Capabilities returns the capabilities of the processor
func (p *planAttributeExtractor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start starts the processor
func (p *planAttributeExtractor) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting plan attribute extractor processor",
		zap.Bool("safe_mode", p.config.SafeMode),
		zap.Bool("unsafe_plan_collection", p.config.UnsafePlanCollection))
	
	if !p.config.SafeMode {
		p.logger.Warn("Plan attribute extractor is not in safe mode - this may impact database performance")
	}
	
	if p.config.UnsafePlanCollection {
		p.logger.Error("UNSAFE: Direct plan collection is enabled - this can severely impact production databases")
	}
	
	// Warn about pg_querylens dependency
	p.logger.Warn("Plan attribute extraction requires pre-collected plan data",
		zap.String("recommendation", "Use pg_stat_statements or similar for safe plan collection"),
		zap.String("unsafe_alternative", "pg_querylens extension (requires C compilation and PostgreSQL restart)"))
	
	return nil
}

// Shutdown stops the processor
func (p *planAttributeExtractor) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down plan attribute extractor processor")
	return nil
}

// ConsumeLogs processes log records and extracts plan attributes
func (p *planAttributeExtractor) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)
				
				// Process each log record with timeout protection
				if err := p.processLogRecord(ctx, logRecord); err != nil {
					if p.config.ErrorMode == "propagate" {
						return fmt.Errorf("failed to process log record: %w", err)
					}
					// In ignore mode, log the error but continue
					p.logger.Warn("Failed to extract plan attributes", 
						zap.Error(err),
						zap.String("mode", "ignore"))
				}
			}
		}
	}
	
	// Forward the processed logs
	return p.consumer.ConsumeLogs(ctx, logs)
}

// processLogRecord extracts attributes from a single log record
func (p *planAttributeExtractor) processLogRecord(ctx context.Context, record plog.LogRecord) error {
	// Create timeout context for safety
	timeoutCtx, cancel := context.WithTimeout(ctx, p.config.GetTimeout())
	defer cancel()
	
	// Check if this record contains plan data
	planData, planType := p.detectPlanType(record)
	if planData == "" {
		// No plan data found, log debug message if enabled
		if p.config.EnableDebugLogging {
			p.logger.Debug("No plan data found in record - this is normal if using pg_stat_statements or similar basic collectors")
		}
		return nil
	}
	
	if p.config.EnableDebugLogging {
		p.logger.Debug("Processing plan data",
			zap.String("plan_type", planType),
			zap.String("plan_preview", p.truncateString(planData, 200)))
	}
	
	// Extract attributes based on plan type
	var extractedAttrs map[string]interface{}
	var err error
	
	switch planType {
	case "postgresql":
		extractedAttrs, err = p.extractPostgreSQLAttributes(timeoutCtx, planData)
	case "mysql":
		extractedAttrs, err = p.extractMySQLAttributes(timeoutCtx, planData)
	default:
		p.logger.Debug("Unknown plan type, skipping extraction", zap.String("type", planType))
		return nil
	}
	
	if err != nil {
		return fmt.Errorf("failed to extract %s attributes: %w", planType, err)
	}
	
	// Apply extracted attributes to the log record
	p.applyAttributes(record, extractedAttrs)
	
	// Generate plan hash for deduplication
	if p.config.HashConfig.Output != "" {
		hash, err := p.generatePlanHash(record)
		if err != nil {
			p.logger.Warn("Failed to generate plan hash", zap.Error(err))
		} else {
			record.Attributes().PutStr(p.config.HashConfig.Output, hash)
		}
	}
	
	return nil
}

// detectPlanType examines the log record to determine if it contains plan data and what type
func (p *planAttributeExtractor) detectPlanType(record plog.LogRecord) (string, string) {
	// Check for PostgreSQL plan in various locations
	if planJson := p.getAttributeAsString(record, "plan_json"); planJson != "" {
		if gjson.Get(planJson, p.config.PostgreSQLRules.DetectionJSONPath).Exists() {
			return planJson, "postgresql"
		}
	}
	
	// Check in body
	if record.Body().Type() == pcommon.ValueTypeStr {
		bodyStr := record.Body().Str()
		if gjson.Get(bodyStr, p.config.PostgreSQLRules.DetectionJSONPath).Exists() {
			return bodyStr, "postgresql"
		}
		if gjson.Get(bodyStr, p.config.MySQLRules.DetectionJSONPath).Exists() {
			return bodyStr, "mysql"
		}
	}
	
	// Check for MySQL metadata
	if gjson.Get(record.Body().AsString(), p.config.MySQLRules.DetectionJSONPath).Exists() {
		return record.Body().AsString(), "mysql"
	}
	
	return "", ""
}

// extractPostgreSQLAttributes extracts attributes from PostgreSQL JSON plans
func (p *planAttributeExtractor) extractPostgreSQLAttributes(ctx context.Context, planData string) (map[string]interface{}, error) {
	attributes := make(map[string]interface{})
	
	// Parse JSON and extract configured attributes
	for attrName, jsonPath := range p.config.PostgreSQLRules.Extractions {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout during PostgreSQL attribute extraction")
		default:
		}
		
		result := gjson.Get(planData, jsonPath)
		if result.Exists() {
			attributes[attrName] = p.convertGJSONValue(result)
		}
	}
	
	// Calculate derived attributes
	for attrName, formula := range p.config.PostgreSQLRules.DerivedAttributes {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout during derived attribute calculation")
		default:
		}
		
		value, err := p.calculateDerivedAttribute(formula, planData, attributes)
		if err != nil {
			p.logger.Warn("Failed to calculate derived attribute",
				zap.String("attribute", attrName),
				zap.String("formula", formula),
				zap.Error(err))
			continue
		}
		attributes[attrName] = value
	}
	
	return attributes, nil
}

// extractMySQLAttributes extracts attributes from MySQL metadata
func (p *planAttributeExtractor) extractMySQLAttributes(ctx context.Context, planData string) (map[string]interface{}, error) {
	attributes := make(map[string]interface{})
	
	for attrName, jsonPath := range p.config.MySQLRules.Extractions {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout during MySQL attribute extraction")
		default:
		}
		
		result := gjson.Get(planData, jsonPath)
		if result.Exists() {
			attributes[attrName] = p.convertGJSONValue(result)
		}
	}
	
	return attributes, nil
}

// calculateDerivedAttribute computes derived attributes using simple formulas
func (p *planAttributeExtractor) calculateDerivedAttribute(formula, planData string, extractedAttrs map[string]interface{}) (interface{}, error) {
	switch formula {
	case "has_substr_in_plan(plan_json, 'Seq Scan')":
		return strings.Contains(planData, "Seq Scan"), nil
	case "has_substr_in_plan(plan_json, 'Nested Loop')":
		return strings.Contains(planData, "Nested Loop"), nil
	case "has_substr_in_plan(plan_json, 'Hash Join')":
		return strings.Contains(planData, "Hash Join"), nil
	case "has_substr_in_plan(plan_json, 'Sort')":
		return strings.Contains(planData, "Sort"), nil
	case "json_depth(plan_json)":
		return p.calculateJSONDepth(planData), nil
	case "json_node_count(plan_json)":
		return p.calculateNodeCount(planData), nil
	case "calculate_efficiency(cost, rows)":
		return p.calculateEfficiency(extractedAttrs), nil
	default:
		return nil, fmt.Errorf("unknown formula: %s", formula)
	}
}

// calculateJSONDepth calculates the depth of a JSON plan
func (p *planAttributeExtractor) calculateJSONDepth(planData string) int {
	depth := 0
	maxDepth := 0
	inString := false
	escaped := false
	
	for _, char := range planData {
		if escaped {
			escaped = false
			continue
		}
		
		switch char {
		case '\\':
			escaped = true
		case '"':
			inString = !inString
		case '{', '[':
			if !inString {
				depth++
				if depth > maxDepth {
					maxDepth = depth
				}
			}
		case '}', ']':
			if !inString {
				depth--
			}
		}
	}
	
	return maxDepth
}

// calculateNodeCount counts the number of plan nodes
func (p *planAttributeExtractor) calculateNodeCount(planData string) int {
	nodeTypeCount := 0
	result := gjson.Get(planData, "$..Node Type")
	result.ForEach(func(key, value gjson.Result) bool {
		nodeTypeCount++
		return true
	})
	return nodeTypeCount
}

// calculateEfficiency calculates a simple efficiency metric
func (p *planAttributeExtractor) calculateEfficiency(attrs map[string]interface{}) float64 {
	cost, costOk := attrs["db.query.plan.cost"].(float64)
	rows, rowsOk := attrs["db.query.plan.rows"].(float64)
	
	if !costOk || !rowsOk || rows == 0 {
		return 0.0
	}
	
	// Simple efficiency: rows per unit cost
	return rows / cost
}

// generatePlanHash generates a hash for the plan based on configured attributes
func (p *planAttributeExtractor) generatePlanHash(record plog.LogRecord) (string, error) {
	var hashInput strings.Builder
	
	// Sort attributes for consistent hashing
	sort.Strings(p.config.HashConfig.Include)
	
	for _, attrName := range p.config.HashConfig.Include {
		value := p.getAttributeAsString(record, attrName)
		hashInput.WriteString(attrName)
		hashInput.WriteString("=")
		hashInput.WriteString(value)
		hashInput.WriteString("|")
	}
	
	// Create appropriate hasher
	var hasher hash.Hash
	switch p.config.HashConfig.Algorithm {
	case "sha256":
		hasher = sha256.New()
	case "sha1":
		hasher = sha1.New()
	case "md5":
		hasher = md5.New()
	default:
		hasher = sha256.New() // Default to SHA256
	}
	
	hasher.Write([]byte(hashInput.String()))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// applyAttributes applies extracted attributes to the log record
func (p *planAttributeExtractor) applyAttributes(record plog.LogRecord, attributes map[string]interface{}) {
	for name, value := range attributes {
		switch v := value.(type) {
		case string:
			record.Attributes().PutStr(name, v)
		case int:
			record.Attributes().PutInt(name, int64(v))
		case int64:
			record.Attributes().PutInt(name, v)
		case float64:
			record.Attributes().PutDouble(name, v)
		case bool:
			record.Attributes().PutBool(name, v)
		default:
			// Convert to string for unknown types
			record.Attributes().PutStr(name, fmt.Sprintf("%v", v))
		}
	}
}

// Helper functions

func (p *planAttributeExtractor) getAttributeAsString(record plog.LogRecord, attrName string) string {
	if attr, exists := record.Attributes().Get(attrName); exists {
		return attr.AsString()
	}
	return ""
}

func (p *planAttributeExtractor) convertGJSONValue(result gjson.Result) interface{} {
	switch result.Type {
	case gjson.String:
		return result.String()
	case gjson.Number:
		if result.Int() == int64(result.Float()) {
			return result.Int()
		}
		return result.Float()
	case gjson.True, gjson.False:
		return result.Bool()
	default:
		return result.String()
	}
}

func (p *planAttributeExtractor) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}