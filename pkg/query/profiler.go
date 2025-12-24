// Package query provides a fluent query builder for database operations.
package query

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProfilerOptions configures the profiler behavior.
type ProfilerOptions struct {
	// SlowThreshold marks queries slower than this as slow.
	SlowThreshold time.Duration
	// EnableCallerInfo captures file:line where queries originate.
	EnableCallerInfo bool
	// MaxProfiles limits stored profiles (0 = unlimited).
	MaxProfiles int
	// NPlusOneThreshold triggers warning if same query runs this many times.
	NPlusOneThreshold int
}

// DefaultProfilerOptions returns sensible defaults.
func DefaultProfilerOptions() ProfilerOptions {
	return ProfilerOptions{
		SlowThreshold:     100 * time.Millisecond,
		EnableCallerInfo:  true,
		MaxProfiles:       10000,
		NPlusOneThreshold: 5,
	}
}

// QueryProfile holds detailed metrics for a single query execution.
type QueryProfile struct {
	// SQL is the executed query.
	SQL string
	// Args are the query parameters.
	Args []interface{}
	// Duration is the execution time.
	Duration time.Duration
	// RowsAffected for INSERT/UPDATE/DELETE.
	RowsAffected int64
	// RowsReturned for SELECT queries.
	RowsReturned int
	// StartTime when the query began.
	StartTime time.Time
	// EndTime when the query completed.
	EndTime time.Time
	// CallerInfo is file:line where query originated.
	CallerInfo string
	// Error if the query failed.
	Error error
	// Tags are user-defined labels.
	Tags []string
	// IsSlow indicates if query exceeded slow threshold.
	IsSlow bool
}

// ProfilingSession tracks a window of profiled queries.
type ProfilingSession struct {
	// ID is a unique session identifier.
	ID string
	// StartTime when profiling began.
	StartTime time.Time
	// EndTime when profiling stopped (zero if active).
	EndTime time.Time
	// Profiles collected during this session.
	Profiles []*QueryProfile
}

// IsActive returns true if the session is still running.
func (s *ProfilingSession) IsActive() bool {
	return s.EndTime.IsZero()
}

// Duration returns how long the session has been running.
func (s *ProfilingSession) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// NPlusOneWarning represents a detected N+1 query pattern.
type NPlusOneWarning struct {
	// Pattern is the normalized SQL pattern.
	Pattern string
	// Count is how many times this pattern was executed.
	Count int
	// Examples are sample queries matching this pattern.
	Examples []string
	// Callers are the source locations where these queries originated.
	Callers []string
}

// ProfileReport contains analysis of a profiling session.
type ProfileReport struct {
	// SessionID of the analyzed session.
	SessionID string
	// TotalQueries executed during the session.
	TotalQueries int
	// TotalDuration of all queries combined.
	TotalDuration time.Duration
	// AverageDuration per query.
	AverageDuration time.Duration
	// SlowQueries that exceeded the threshold.
	SlowQueries []*QueryProfile
	// TopByDuration are the slowest queries.
	TopByDuration []*QueryProfile
	// TopByFrequency shows query patterns and their execution counts.
	TopByFrequency []QueryFrequency
	// NPlusOneWarnings for detected N+1 patterns.
	NPlusOneWarnings []NPlusOneWarning
	// Suggestions for performance improvements.
	Suggestions []string
	// ErrorCount is the number of failed queries.
	ErrorCount int
	// SessionDuration is the total profiling window.
	SessionDuration time.Duration
}

// QueryFrequency tracks how often a query pattern was executed.
type QueryFrequency struct {
	Pattern       string
	Count         int
	TotalDuration time.Duration
	AvgDuration   time.Duration
}

// Profiler manages performance profiling sessions.
type Profiler struct {
	mu      sync.RWMutex
	opts    ProfilerOptions
	session *ProfilingSession
	enabled bool
}

// NewProfiler creates a new profiler with the given options.
func NewProfiler(opts ProfilerOptions) *Profiler {
	return &Profiler{
		opts:    opts,
		enabled: false,
	}
}

// Start begins a new profiling session.
func (p *Profiler) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.session = &ProfilingSession{
		ID:        fmt.Sprintf("session_%d", time.Now().UnixNano()),
		StartTime: time.Now(),
		Profiles:  make([]*QueryProfile, 0, 100),
	}
	p.enabled = true
}

// Stop ends the current profiling session.
func (p *Profiler) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.session != nil {
		p.session.EndTime = time.Now()
	}
	p.enabled = false
}

// IsEnabled returns true if profiling is active.
func (p *Profiler) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

// StartQuery begins profiling a query and returns a profile to be completed.
func (p *Profiler) StartQuery(sql string, args []interface{}) *QueryProfile {
	profile := &QueryProfile{
		SQL:       sql,
		Args:      args,
		StartTime: time.Now(),
	}

	if p.opts.EnableCallerInfo {
		profile.CallerInfo = getCallerInfo(4) // Skip internal frames
	}

	return profile
}

// EndQuery completes a query profile and records it.
func (p *Profiler) EndQuery(profile *QueryProfile, err error) {
	profile.EndTime = time.Now()
	profile.Duration = profile.EndTime.Sub(profile.StartTime)
	profile.Error = err
	profile.IsSlow = profile.Duration > p.opts.SlowThreshold

	p.Record(profile)
}

// Record adds a completed query profile to the session.
func (p *Profiler) Record(profile *QueryProfile) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.session == nil || !p.enabled {
		return
	}

	// Enforce max profiles limit
	if p.opts.MaxProfiles > 0 && len(p.session.Profiles) >= p.opts.MaxProfiles {
		// Remove oldest profile
		p.session.Profiles = p.session.Profiles[1:]
	}

	p.session.Profiles = append(p.session.Profiles, profile)
}

// Tag adds tags to the current context for subsequent queries.
func (p *Profiler) Tag(tags ...string) *Profiler {
	// Tags are applied per-query via context in real usage
	return p
}

// Reset clears all collected profiles.
func (p *Profiler) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.session != nil {
		p.session.Profiles = make([]*QueryProfile, 0, 100)
	}
}

// Report generates an analysis report from the current session.
func (p *Profiler) Report() *ProfileReport {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.session == nil {
		return &ProfileReport{}
	}

	report := &ProfileReport{
		SessionID:       p.session.ID,
		TotalQueries:    len(p.session.Profiles),
		SessionDuration: p.session.Duration(),
	}

	if report.TotalQueries == 0 {
		return report
	}

	// Calculate totals
	var totalDuration time.Duration
	patternCounts := make(map[string]*patternStats)

	for _, profile := range p.session.Profiles {
		totalDuration += profile.Duration

		if profile.IsSlow {
			report.SlowQueries = append(report.SlowQueries, profile)
		}

		if profile.Error != nil {
			report.ErrorCount++
		}

		// Normalize SQL for pattern matching
		pattern := normalizeSQL(profile.SQL)
		if stats, ok := patternCounts[pattern]; ok {
			stats.count++
			stats.totalDuration += profile.Duration
			if len(stats.examples) < 3 {
				stats.examples = append(stats.examples, profile.SQL)
			}
			if profile.CallerInfo != "" && len(stats.callers) < 3 {
				stats.callers = append(stats.callers, profile.CallerInfo)
			}
		} else {
			patternCounts[pattern] = &patternStats{
				count:         1,
				totalDuration: profile.Duration,
				examples:      []string{profile.SQL},
				callers:       []string{profile.CallerInfo},
			}
		}
	}

	report.TotalDuration = totalDuration
	if report.TotalQueries > 0 {
		report.AverageDuration = totalDuration / time.Duration(report.TotalQueries)
	}

	// Top by duration (slowest queries)
	sortedByDuration := make([]*QueryProfile, len(p.session.Profiles))
	copy(sortedByDuration, p.session.Profiles)
	sort.Slice(sortedByDuration, func(i, j int) bool {
		return sortedByDuration[i].Duration > sortedByDuration[j].Duration
	})
	if len(sortedByDuration) > 10 {
		sortedByDuration = sortedByDuration[:10]
	}
	report.TopByDuration = sortedByDuration

	// Top by frequency
	for pattern, stats := range patternCounts {
		report.TopByFrequency = append(report.TopByFrequency, QueryFrequency{
			Pattern:       pattern,
			Count:         stats.count,
			TotalDuration: stats.totalDuration,
			AvgDuration:   stats.totalDuration / time.Duration(stats.count),
		})
	}
	sort.Slice(report.TopByFrequency, func(i, j int) bool {
		return report.TopByFrequency[i].Count > report.TopByFrequency[j].Count
	})
	if len(report.TopByFrequency) > 10 {
		report.TopByFrequency = report.TopByFrequency[:10]
	}

	// Detect N+1 patterns
	for pattern, stats := range patternCounts {
		if stats.count >= p.opts.NPlusOneThreshold {
			report.NPlusOneWarnings = append(report.NPlusOneWarnings, NPlusOneWarning{
				Pattern:  pattern,
				Count:    stats.count,
				Examples: stats.examples,
				Callers:  stats.callers,
			})
		}
	}

	// Generate suggestions
	report.Suggestions = p.generateSuggestions(report, patternCounts)

	return report
}

// patternStats holds statistics for a query pattern.
type patternStats struct {
	count         int
	totalDuration time.Duration
	examples      []string
	callers       []string
}

// generateSuggestions creates optimization suggestions based on the report.
func (p *Profiler) generateSuggestions(report *ProfileReport, patterns map[string]*patternStats) []string {
	var suggestions []string

	// N+1 detection
	if len(report.NPlusOneWarnings) > 0 {
		for _, warning := range report.NPlusOneWarnings {
			suggestions = append(suggestions,
				fmt.Sprintf("🔁 N+1 detected: Query pattern executed %d times - consider using eager loading or batch queries:\n   %s",
					warning.Count, truncateSQL(warning.Pattern, 80)))
		}
	}

	// Slow queries
	if len(report.SlowQueries) > 0 {
		slowCount := len(report.SlowQueries)
		slowPct := float64(slowCount) / float64(report.TotalQueries) * 100
		if slowPct > 10 {
			suggestions = append(suggestions,
				fmt.Sprintf("🐢 %.1f%% of queries (%d) are slow - consider adding indexes or optimizing queries",
					slowPct, slowCount))
		}
	}

	// High query count
	if report.TotalQueries > 100 && report.SessionDuration < 5*time.Second {
		suggestions = append(suggestions,
			fmt.Sprintf("📊 High query volume: %d queries in %s - consider caching or batch operations",
				report.TotalQueries, report.SessionDuration.Round(time.Millisecond)))
	}

	// SELECT * detection
	for pattern := range patterns {
		if strings.Contains(strings.ToUpper(pattern), "SELECT *") {
			suggestions = append(suggestions,
				"📋 Detected SELECT * - consider selecting only needed columns for better performance")
			break
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "✅ No obvious performance issues detected")
	}

	return suggestions
}

// String returns a formatted text report.
func (r *ProfileReport) String() string {
	var sb strings.Builder

	sb.WriteString("\n📊 Performance Profile Report\n")
	sb.WriteString(strings.Repeat("─", 50) + "\n\n")

	sb.WriteString(fmt.Sprintf("Session:          %s\n", r.SessionID))
	sb.WriteString(fmt.Sprintf("Duration:         %s\n", r.SessionDuration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("Total Queries:    %d\n", r.TotalQueries))
	sb.WriteString(fmt.Sprintf("Total Time:       %s\n", r.TotalDuration.Round(time.Microsecond)))
	sb.WriteString(fmt.Sprintf("Avg Query Time:   %s\n", r.AverageDuration.Round(time.Microsecond)))
	sb.WriteString(fmt.Sprintf("Slow Queries:     %d\n", len(r.SlowQueries)))
	sb.WriteString(fmt.Sprintf("Errors:           %d\n", r.ErrorCount))

	if len(r.TopByDuration) > 0 {
		sb.WriteString("\n🐢 Slowest Queries:\n")
		for i, q := range r.TopByDuration {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("   %d. [%s] %s\n", i+1,
				q.Duration.Round(time.Microsecond),
				truncateSQL(q.SQL, 60)))
			if q.CallerInfo != "" {
				sb.WriteString(fmt.Sprintf("      └─ %s\n", q.CallerInfo))
			}
		}
	}

	if len(r.TopByFrequency) > 0 {
		sb.WriteString("\n🔄 Most Frequent Queries:\n")
		for i, f := range r.TopByFrequency {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("   %d. [%dx, avg %s] %s\n", i+1,
				f.Count,
				f.AvgDuration.Round(time.Microsecond),
				truncateSQL(f.Pattern, 50)))
		}
	}

	if len(r.NPlusOneWarnings) > 0 {
		sb.WriteString("\n⚠️  N+1 Query Warnings:\n")
		for _, w := range r.NPlusOneWarnings {
			sb.WriteString(fmt.Sprintf("   • %dx: %s\n", w.Count, truncateSQL(w.Pattern, 60)))
			if len(w.Callers) > 0 {
				sb.WriteString(fmt.Sprintf("     └─ from: %s\n", w.Callers[0]))
			}
		}
	}

	if len(r.Suggestions) > 0 {
		sb.WriteString("\n💡 Suggestions:\n")
		for _, s := range r.Suggestions {
			sb.WriteString(fmt.Sprintf("   %s\n", s))
		}
	}

	sb.WriteString("\n" + strings.Repeat("─", 50) + "\n")

	return sb.String()
}

// getCallerInfo returns the file:line of the caller.
func getCallerInfo(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}

	// Shorten file path
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		file = strings.Join(parts[len(parts)-2:], "/")
	}

	return fmt.Sprintf("%s:%d", file, line)
}

// normalizeSQL removes literal values to create a pattern.
func normalizeSQL(sql string) string {
	// Simple normalization: replace quoted strings and numbers
	result := sql

	// Replace string literals
	inQuote := false
	var normalized strings.Builder
	for i := 0; i < len(result); i++ {
		c := result[i]
		if c == '\'' && (i == 0 || result[i-1] != '\\') {
			if !inQuote {
				normalized.WriteString("?")
			}
			inQuote = !inQuote
			continue
		}
		if !inQuote {
			normalized.WriteByte(c)
		}
	}

	return strings.TrimSpace(normalized.String())
}

// truncateSQL shortens SQL for display.
func truncateSQL(sql string, maxLen int) string {
	// Remove extra whitespace
	sql = strings.Join(strings.Fields(sql), " ")
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen-3] + "..."
}

// ProfilerContext is a context key for profiler.
type profilerContextKey struct{}

// WithProfiler adds a profiler to the context.
func WithProfilerContext(ctx context.Context, p *Profiler) context.Context {
	return context.WithValue(ctx, profilerContextKey{}, p)
}

// ProfilerFromContext retrieves the profiler from context.
func ProfilerFromContext(ctx context.Context) *Profiler {
	if p, ok := ctx.Value(profilerContextKey{}).(*Profiler); ok {
		return p
	}
	return nil
}
