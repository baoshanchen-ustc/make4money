package runtimeprobe

import "github.com/Wei-Shaw/sub2api/internal/repository"

type DBPoolSnapshot = repository.DBPoolSnapshot
type RedisPoolSnapshot = repository.RedisPoolSnapshot
type HTTPUpstreamRuntimeSnapshot = repository.HTTPUpstreamRuntimeSnapshot
type StickySessionCompareDeleteMetricsSnapshot = repository.StickySessionCompareDeleteMetricsSnapshot

func SnapshotDefaultDBPoolStats() DBPoolSnapshot {
	return repository.SnapshotDefaultDBPoolStats()
}

func SnapshotDefaultRedisPoolStats() RedisPoolSnapshot {
	return repository.SnapshotDefaultRedisPoolStats()
}

func SnapshotDefaultHTTPUpstreamRuntime() HTTPUpstreamRuntimeSnapshot {
	return repository.SnapshotDefaultHTTPUpstreamRuntime()
}

func SnapshotStickySessionCompareDeleteMetrics() StickySessionCompareDeleteMetricsSnapshot {
	return repository.SnapshotStickySessionCompareDeleteMetrics()
}

func EvaluateRedisPressure(snapshot RedisPoolSnapshot) (string, string) {
	return repository.EvaluateRedisPressure(snapshot)
}

func EvaluateHTTPUpstreamPressure(snapshot HTTPUpstreamRuntimeSnapshot) (string, string) {
	return repository.EvaluateHTTPUpstreamPressure(snapshot)
}
