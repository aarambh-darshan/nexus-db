package query

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the logging level.
type LogLevel int

const (
	LogSilent LogLevel = iota
	LogError
	LogWarn
	LogInfo
	LogDebug
)

// Logger is the interface for query logging.
type Logger interface {
	Log(level LogLevel, msg string, fields map[string]interface{})
}

// DefaultLogger is a simple logger implementation.
type DefaultLogger struct {
	level  LogLevel
	logger *log.Logger
	mask   bool // Mask sensitive values
}

// NewLogger creates a new default logger.
func NewLogger(w io.Writer, level LogLevel) *DefaultLogger {
	if w == nil {
		w = os.Stdout
	}
	return &DefaultLogger{
		level:  level,
		logger: log.New(w, "[NEXUS] ", log.LstdFlags|log.Lmicroseconds),
		mask:   false,
	}
}

// NewMaskedLogger creates a logger that masks parameter values.
func NewMaskedLogger(w io.Writer, level LogLevel) *DefaultLogger {
	l := NewLogger(w, level)
	l.mask = true
	return l
}

// Log logs a message at the given level.
func (l *DefaultLogger) Log(level LogLevel, msg string, fields map[string]interface{}) {
	if level > l.level {
		return
	}

	levelStr := "INFO"
	switch level {
	case LogError:
		levelStr = "ERROR"
	case LogWarn:
		levelStr = "WARN"
	case LogDebug:
		levelStr = "DEBUG"
	}

	// Format fields
	var parts []string
	for k, v := range fields {
		if l.mask && k == "args" {
			parts = append(parts, k+"=[MASKED]")
		} else {
			parts = append(parts, k+"="+formatValue(v))
		}
	}

	fieldStr := ""
	if len(parts) > 0 {
		fieldStr = " " + strings.Join(parts, " ")
	}

	l.logger.Printf("[%s] %s%s", levelStr, msg, fieldStr)
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return `"` + val + `"`
	case time.Duration:
		return val.String()
	case []interface{}:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// QueryLogger wraps a connection with logging.
type QueryLogger struct {
	logger             Logger
	slowQueryThreshold time.Duration
}

// NewQueryLogger creates a new query logger.
func NewQueryLogger(logger Logger) *QueryLogger {
	return &QueryLogger{
		logger:             logger,
		slowQueryThreshold: 100 * time.Millisecond,
	}
}

// SetSlowQueryThreshold sets the threshold for slow query warnings.
func (q *QueryLogger) SetSlowQueryThreshold(d time.Duration) {
	q.slowQueryThreshold = d
}

// LogQuery logs a query execution.
func (q *QueryLogger) LogQuery(ctx context.Context, sql string, args []interface{}, duration time.Duration, err error) {
	fields := map[string]interface{}{
		"sql":      sql,
		"duration": duration,
	}

	if len(args) > 0 {
		fields["args"] = args
	}

	if err != nil {
		fields["error"] = err.Error()
		q.logger.Log(LogError, "query failed", fields)
		return
	}

	if duration > q.slowQueryThreshold {
		q.logger.Log(LogWarn, "slow query", fields)
		return
	}

	q.logger.Log(LogDebug, "query executed", fields)
}

// LogQueryStart logs the start of a query for timing.
func (q *QueryLogger) LogQueryStart(sql string) time.Time {
	return time.Now()
}

// LogQueryEnd logs the end of a query with timing.
func (q *QueryLogger) LogQueryEnd(ctx context.Context, sql string, args []interface{}, start time.Time, err error) {
	q.LogQuery(ctx, sql, args, time.Since(start), err)
}

// QueryStats holds query execution statistics.
type QueryStats struct {
	TotalQueries  int64
	TotalDuration time.Duration
	SlowQueries   int64
	Errors        int64
	LastQuery     string
	LastQueryAt   time.Time
}

// StatsCollector collects query statistics.
type StatsCollector struct {
	stats         QueryStats
	slowThreshold time.Duration
}

// NewStatsCollector creates a new statistics collector.
func NewStatsCollector(slowThreshold time.Duration) *StatsCollector {
	return &StatsCollector{
		slowThreshold: slowThreshold,
	}
}

// Record records a query execution.
func (s *StatsCollector) Record(sql string, duration time.Duration, err error) {
	s.stats.TotalQueries++
	s.stats.TotalDuration += duration
	s.stats.LastQuery = sql
	s.stats.LastQueryAt = time.Now()

	if err != nil {
		s.stats.Errors++
	}

	if duration > s.slowThreshold {
		s.stats.SlowQueries++
	}
}

// Stats returns the current statistics.
func (s *StatsCollector) Stats() QueryStats {
	return s.stats
}

// AverageQueryTime returns the average query execution time.
func (s *StatsCollector) AverageQueryTime() time.Duration {
	if s.stats.TotalQueries == 0 {
		return 0
	}
	return s.stats.TotalDuration / time.Duration(s.stats.TotalQueries)
}

// Reset resets all statistics.
func (s *StatsCollector) Reset() {
	s.stats = QueryStats{}
}
