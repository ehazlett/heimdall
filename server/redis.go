package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	redisConfTemplate = `# heimdall redis
dir {{ .DataDir }}
bind {{ .ListenAddr }}
port {{ .Port }}
protected-mode no
timeout 0
tcp-keepalive 300
daemonize no
supervised no
databases 1
save 900 1
save 300 10
save 60 1000
stop-writes-on-bgsave-error yes
rdbcompression yes
rdbchecksum yes
dbfilename dump.rdb
replica-serve-stale-data yes
replica-read-only yes
repl-diskless-sync no
repl-diskless-sync-delay 5
repl-disable-tcp-nodelay no
replica-priority 100
lazyfree-lazy-eviction no
lazyfree-lazy-expire no
lazyfree-lazy-server-del no
replica-lazy-flush no
appendonly no
no-appendfsync-on-rewrite no
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb
aof-load-truncated yes
aof-use-rdb-preamble yes
lua-time-limit 5000
slowlog-log-slower-than 10000
slowlog-max-len 128
latency-monitor-threshold 0
notify-keyspace-events ""
activerehashing yes
client-output-buffer-limit normal 0 0 0
client-output-buffer-limit replica 256mb 64mb 60
client-output-buffer-limit pubsub 32mb 8mb 60
hz 10
dynamic-hz yes
{{ if .ReplicaOf }}replicaof {{ .ReplicaOf.Host }} {{ .ReplicaOf.Port }} {{ end }}
`
)

type redisReplica struct {
	Host string
	Port int
}

type redisConfig struct {
	ListenAddr string
	Port       int
	DataDir    string
	ReplicaOf  *redisReplica
}

// startRedis starts and embedded redis and returns the exec.Cmd
func startRedis(ctx context.Context, cfg *redisConfig) (*exec.Cmd, error) {
	redisCmdPath, err := exec.LookPath("redis-server")
	if err != nil {
		return nil, err
	}
	// write out config
	redisConfPath := filepath.Join(cfg.DataDir, "redis.conf")
	if _, err := os.Stat(redisConfPath); err == nil {
		if err := os.Remove(redisConfPath); err != nil {
			return nil, err
		}
	}
	f, err := os.Create(redisConfPath)
	if err != nil {
		return nil, err
	}

	t, err := template.New("redis-conf").Parse(redisConfTemplate)
	if err != nil {
		return nil, err
	}

	if err := t.Execute(f, cfg); err != nil {
		return nil, err
	}

	f.Close()

	logrus.Debugf("starting redis on %s with port %d with path %s", cfg.ListenAddr, cfg.Port, redisConfPath)
	cmd := exec.CommandContext(ctx, redisCmdPath, redisConfPath)
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// TODO: add a loop to wait on a successful redis command before returning to ensure server is up
	time.Sleep(time.Second * 1)

	return cmd, nil
}
