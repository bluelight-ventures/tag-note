package admin

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/gofiber/fiber/v2"
)

var (
	startTime = time.Now()

	requestCounters   = make(map[string]*metrics.Counter)
	requestHistograms = make(map[string]*metrics.Histogram)
	mu                sync.RWMutex

	registeredUsersGauge  = metrics.NewGauge("app_registered_users", nil)
	activeUsersDailyGauge = metrics.NewGauge("app_active_users_daily", nil)
	notesTotalGauge       = metrics.NewGauge("app_notes_total", nil)
	uptimeSecondsGauge    = metrics.NewGauge("app_uptime_seconds", func() float64 {
		return time.Since(startTime).Seconds()
	})

	pathParamPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^/api/v1/notes/([a-zA-Z0-9]+)$`),
		regexp.MustCompile(`^/api/v1/notes/([a-zA-Z0-9]+)/pin$`),
		regexp.MustCompile(`^/api/v1/notes/([a-zA-Z0-9]+)/restore$`),
		regexp.MustCompile(`^/api/v1/notes/([a-zA-Z0-9]+)/permanent$`),
		regexp.MustCompile(`^/api/v1/tags/([^/]+)/approve$`),
		regexp.MustCompile(`^/api/v1/tags/([^/]+)/rename$`),
		regexp.MustCompile(`^/api/v1/tags/([^/]+)/priority$`),
		regexp.MustCompile(`^/api/v1/tags/([^/]+)$`),
	}
	pathReplacements = []string{
		"/api/v1/notes/:id",
		"/api/v1/notes/:id/pin",
		"/api/v1/notes/:id/restore",
		"/api/v1/notes/:id/permanent",
		"/api/v1/tags/:name/approve",
		"/api/v1/tags/:name/rename",
		"/api/v1/tags/:name/priority",
		"/api/v1/tags/:name",
	}
)

// normalizePath normalizes a path by replacing dynamic segments with placeholders
// to avoid cardinality explosion in metrics.
func normalizePath(path string) string {
	for i, pattern := range pathParamPatterns {
		if pattern.MatchString(path) {
			return pathReplacements[i]
		}
	}
	return path
}

// getCounter returns a counter for the given method, path, and status.
func getCounter(method, path string, status int) *metrics.Counter {
	key := method + "|" + path + "|" + strconv.Itoa(status)
	mu.RLock()
	counter, exists := requestCounters[key]
	mu.RUnlock()
	if exists {
		return counter
	}

	mu.Lock()
	defer mu.Unlock()
	if counter, exists = requestCounters[key]; exists {
		return counter
	}

	name := `http_requests_total{method="` + method + `",path="` + path + `",status="` + strconv.Itoa(status) + `"}`
	counter = metrics.NewCounter(name)
	requestCounters[key] = counter
	return counter
}

// getHistogram returns a histogram for the given method and path.
func getHistogram(method, path string) *metrics.Histogram {
	key := method + "|" + path
	mu.RLock()
	histogram, exists := requestHistograms[key]
	mu.RUnlock()
	if exists {
		return histogram
	}

	mu.Lock()
	defer mu.Unlock()
	if histogram, exists = requestHistograms[key]; exists {
		return histogram
	}

	name := `http_request_duration_seconds{method="` + method + `",path="` + path + `"}`
	histogram = metrics.NewHistogram(name)
	requestHistograms[key] = histogram
	return histogram
}

// MetricsMiddleware is a Fiber middleware that records request metrics.
func MetricsMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		path := c.Path()
		if !strings.HasPrefix(path, "/api/") &&
			path != "/metrics" &&
			path != "/healthz" &&
			path != "/status" {
			return err
		}

		method := c.Method()
		status := c.Response().StatusCode()
		normalizedPath := normalizePath(path)

		duration := time.Since(start).Seconds()
		getCounter(method, normalizedPath, status).Inc()
		getHistogram(method, normalizedPath).Update(duration)

		return err
	}
}

// ExposeMetrics is a Fiber handler that exposes Prometheus-compatible metrics.
func ExposeMetrics(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain; charset=utf-8")
	metrics.WritePrometheus(c, false)
	return nil
}

// UpdateGauges updates gauges that require database queries.
// This should be called periodically (e.g., every 30 seconds).
func UpdateGauges(userCount, activeUsersDaily, notesCount int) {
	registeredUsersGauge.Set(float64(userCount))
	activeUsersDailyGauge.Set(float64(activeUsersDaily))
	notesTotalGauge.Set(float64(notesCount))
}
