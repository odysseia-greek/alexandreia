package grammar

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	queuepb "github.com/odysseia-greek/agora/eupalinos/v1"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/middleware"
	pba "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	v1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	sv1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

const dependencyHealthTimeout = time.Second

type kubeHealthResponse struct {
	Status string `json:"status"`
}

// healthz is deliberately local and constant-time: a running HTTP server is alive.
func (d *DionysosHandler) healthz(w http.ResponseWriter, _ *http.Request) {
	middleware.ResponseWithCustomCode(w, http.StatusOK, kubeHealthResponse{Status: "ok"})
}

// readyz performs no network I/O. Detailed dependency state belongs to the Health RPC.
func (d *DionysosHandler) readyz(w http.ResponseWriter, _ *http.Request) {
	_, _, mappingLoaded := d.mappingCounts()
	ready := d.Elastic != nil && d.Cache != nil && mappingLoaded &&
		d.LibraryService != nil && d.LibraryService.Client != nil &&
		d.ScholarService != nil && d.ScholarService.Client != nil &&
		d.AggregatorClient != nil && d.AggregatorQueue != nil
	if !ready {
		middleware.ResponseWithCustomCode(w, http.StatusServiceUnavailable, kubeHealthResponse{Status: "not ready"})
		return
	}

	middleware.ResponseWithCustomCode(w, http.StatusOK, kubeHealthResponse{Status: "ready"})
}

func (d *DionysosHandler) detailedHealth(ctx context.Context) *v1.HealthResponse {
	ctx, cancel := context.WithTimeout(ctx, 2*dependencyHealthTimeout)
	defer cancel()

	declensions, rules, loaded := d.mappingCounts()
	response := &v1.HealthResponse{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Version: d.Version,
		Tracer:  d.tracerHealth(),
		Cache:   d.cacheHealth(),
		Mapping: &v1.MappingHealth{
			Loaded:          loaded,
			DeclensionCount: uint32(declensions),
			RuleCount:       uint32(rules),
		},
	}

	type result struct {
		name      string
		component *v1.ComponentHealth
		database  *v1.DatabaseHealth
	}
	checks := []func() result{
		func() result {
			started := time.Now()
			if d.Elastic == nil {
				return result{name: "elasticsearch", component: unconfiguredComponent("elasticsearch")}
			}
			info := d.Elastic.Health().Info()
			return result{
				name:      "elasticsearch",
				component: component("elasticsearch", info.Healthy, time.Since(started), ""),
				database: &v1.DatabaseHealth{
					Healthy:       info.Healthy,
					ClusterName:   info.ClusterName,
					ServerName:    info.ServerName,
					ServerVersion: info.ServerVersion,
				},
			}
		},
		func() result {
			if d.LibraryService == nil || d.LibraryService.Client == nil {
				return result{name: "eratosthenes", component: unconfiguredComponent("eratosthenes")}
			}
			started := time.Now()
			checkCtx, checkCancel := context.WithTimeout(ctx, dependencyHealthTimeout)
			defer checkCancel()
			value, err := d.LibraryService.Client.Health(checkCtx, &emptypb.Empty{})
			return result{name: "eratosthenes", component: component("eratosthenes", err == nil && value.GetHealthy(), time.Since(started), errorMessage(err))}
		},
		func() result {
			if d.ScholarService == nil || d.ScholarService.Client == nil {
				return result{name: "kallimachos", component: unconfiguredComponent("kallimachos")}
			}
			started := time.Now()
			checkCtx, checkCancel := context.WithTimeout(ctx, dependencyHealthTimeout)
			defer checkCancel()
			value, err := d.ScholarService.Client.Health(checkCtx, &sv1.HealthRequest{})
			return result{name: "kallimachos", component: component("kallimachos", err == nil && value.GetHealthy(), time.Since(started), errorMessage(err))}
		},
		func() result {
			if d.AggregatorClient == nil {
				return result{name: "aristarchos", component: unconfiguredComponent("aristarchos")}
			}
			started := time.Now()
			checkCtx, checkCancel := context.WithTimeout(ctx, dependencyHealthTimeout)
			defer checkCancel()
			value, err := d.AggregatorClient.Health(checkCtx, &pba.HealthRequest{})
			return result{name: "aristarchos", component: component("aristarchos", err == nil && value.GetHealth(), time.Since(started), errorMessage(err))}
		},
		func() result {
			healthClient, ok := d.AggregatorQueue.(interface {
				Health(context.Context, *queuepb.HealthRequest) (*queuepb.HealthResponse, error)
			})
			if !ok || healthClient == nil {
				return result{name: "eupalinos", component: unconfiguredComponent("eupalinos")}
			}
			started := time.Now()
			checkCtx, checkCancel := context.WithTimeout(ctx, dependencyHealthTimeout)
			defer checkCancel()
			value, err := healthClient.Health(checkCtx, &queuepb.HealthRequest{})
			return result{name: "eupalinos", component: component("eupalinos", err == nil && value.GetHealthy(), time.Since(started), errorMessage(err))}
		},
	}

	results := make(chan result, len(checks))
	for _, check := range checks {
		go func(check func() result) { results <- check() }(check)
	}

	components := make(map[string]*v1.ComponentHealth, len(checks))
	for range checks {
		select {
		case check := <-results:
			components[check.name] = check.component
			if check.database != nil {
				response.DatabaseHealth = check.database
				response.Elasticsearch = check.component
			}
		case <-ctx.Done():
			// Preserve a complete, predictably ordered response below.
		}
	}

	for _, name := range []string{"elasticsearch", "eratosthenes", "kallimachos", "aristarchos", "eupalinos"} {
		check := components[name]
		if check == nil {
			check = component(name, false, 2*dependencyHealthTimeout, ctx.Err().Error())
		}
		if name == "elasticsearch" {
			response.Elasticsearch = check
		} else {
			response.Dependencies = append(response.Dependencies, check)
		}
	}
	if response.DatabaseHealth == nil {
		response.DatabaseHealth = &v1.DatabaseHealth{}
	}
	response.Healthy = response.DatabaseHealth.Healthy && response.Tracer.Healthy &&
		response.Cache.Healthy && response.Mapping.Loaded
	for _, dependency := range response.Dependencies {
		response.Healthy = response.Healthy && dependency.Healthy
	}
	d.logHealth(response)

	return response
}

func (d *DionysosHandler) logHealth(health *v1.HealthResponse) {
	states := []string{
		fmt.Sprintf("elasticsearch=%s", health.Elasticsearch.Status),
		fmt.Sprintf("tracer=%s", health.Tracer.Status),
		fmt.Sprintf("cache=%s", health.Cache.Status),
		fmt.Sprintf("mapping_loaded=%t", health.Mapping.Loaded),
	}
	for _, dependency := range health.Dependencies {
		states = append(states, fmt.Sprintf("%s=%s", dependency.Name, dependency.Status))
	}
	message := fmt.Sprintf("dionysios health: healthy=%t %s", health.Healthy, strings.Join(states, " "))
	if health.Healthy {
		logging.Debug(message)
		return
	}
	logging.Error(message)
}

func (d *DionysosHandler) mappingCounts() (declensions, rules int, loaded bool) {
	d.DeclensionMu.RLock()
	defer d.DeclensionMu.RUnlock()
	declensions = len(d.DeclensionConfig.Declensions)
	for _, declension := range d.DeclensionConfig.Declensions {
		rules += len(declension.Declensions)
	}
	return declensions, rules, declensions > 0 && rules > 0
}

func (d *DionysosHandler) tracerHealth() *v1.ComponentHealth {
	if d.Streamer == nil {
		return unconfiguredComponent("tracer")
	}
	err := d.Streamer.Context().Err()
	return component("tracer", err == nil, 0, errorMessage(err))
}

func (d *DionysosHandler) cacheHealth() *v1.CacheHealth {
	if d.Cache == nil {
		return &v1.CacheHealth{Status: "not_configured"}
	}

	stats, err := d.Cache.Stats()
	healthy := err == nil
	message := errorMessage(err)
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}
	return &v1.CacheHealth{
		Configured:        true,
		Healthy:           healthy,
		Implementation:    reflect.TypeOf(d.Cache).String(),
		Status:            status,
		Message:           message,
		KeyCount:          stats.KeyCount,
		LsmSizeBytes:      stats.LSMSizeBytes,
		ValueLogSizeBytes: stats.ValueLogSizeBytes,
		TotalSizeBytes:    stats.TotalSizeBytes,
		BlockCache:        mapCacheMetrics(stats.BlockCache),
		IndexCache:        mapCacheMetrics(stats.IndexCache),
	}
}

func mapCacheMetrics(stats archytas.CacheStats) *v1.CacheMetrics {
	return &v1.CacheMetrics{
		Hits:        stats.Hits,
		Misses:      stats.Misses,
		KeysAdded:   stats.KeysAdded,
		KeysUpdated: stats.KeysUpdated,
		KeysEvicted: stats.KeysEvicted,
		CostAdded:   stats.CostAdded,
		CostEvicted: stats.CostEvicted,
	}
}

func component(name string, healthy bool, latency time.Duration, message string) *v1.ComponentHealth {
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}
	return &v1.ComponentHealth{Name: name, Configured: true, Healthy: healthy, Status: status, Message: message, LatencyMs: uint64(latency.Milliseconds())}
}

func unconfiguredComponent(name string) *v1.ComponentHealth {
	return &v1.ComponentHealth{Name: name, Status: "not_configured", Message: fmt.Sprintf("%s client is not configured", name)}
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
