package queue

import (
	"github.com/hibiken/asynq"
)

// NewWorker creates a new Asynq server (worker) configured with the given
// Redis address. It uses three priority queues: critical (6), default (3),
// and low (1), with a concurrency of 10.
func NewWorker(redisAddr string) *asynq.Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
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

// NewClient creates a new Asynq client connected to the given Redis address.
func NewClient(redisAddr string) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
}
