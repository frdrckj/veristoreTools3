package queue

import (
	"github.com/hibiken/asynq"
)

// RedisConfig holds the Redis connection parameters for the queue system.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// NewWorker creates a new Asynq server (worker) configured with the given
// Redis config. It uses three priority queues: critical (6), default (3),
// and low (1), with a concurrency of 10.
func NewWorker(redisCfg RedisConfig) *asynq.Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisCfg.Addr,
			Password: redisCfg.Password,
			DB:       redisCfg.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)
	return srv
}

// NewMux creates a new Asynq ServeMux and registers the provided handlers
// keyed by their task type pattern.
func NewMux(handlers map[string]asynq.Handler) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	for pattern, handler := range handlers {
		mux.Handle(pattern, handler)
	}
	return mux
}

// NewClient creates a new Asynq client connected to the given Redis config.
func NewClient(redisCfg RedisConfig) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
}

// NewInspector creates a new Asynq inspector for queue management (purge, cancel, etc.).
func NewInspector(redisCfg RedisConfig) *asynq.Inspector {
	return asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
}
