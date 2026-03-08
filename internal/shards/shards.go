package shards

import (
	"hash/fnv"
	"sync"
	"time"
)

type Shard struct {
	mu      sync.RWMutex
	visitor map[string]*visitors
}

type visitors struct {
	visitorID    string
	requestCount int
	lastVisit    time.Time
}

type Job struct {
	key   string
	allow chan bool
}

type Response struct {
	allowed bool
	visitor *visitors
}

type RateLimiter struct {
	shards     []*Shard
	shardCount int
	jobs       chan Job
	response   chan Response
}

func NewRateLimiter(shardCount int) *RateLimiter {
	shardSlice := make([]*Shard, shardCount)
	for i := 0; i < shardCount; i++ {
		shardSlice[i] = &Shard{
			visitor: make(map[string]*visitors),
		}
	}

	rl := &RateLimiter{
		shards:     shardSlice,
		shardCount: shardCount,
		jobs:       make(chan Job, 10000),
		response:   make(chan Response, 10000),
	}

	//start 8 background workers
	for i := 0; i < 8; i++ {
		go rl.Worker()
	}

	return rl
}

// rate limiter functions
func (rl *RateLimiter) GetShard(key string) *Shard {

	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	index := int(hasher.Sum32() % uint32(rl.shardCount))

	shard := rl.shards[index]
	if shard == nil {
		return nil
	}
	return shard
}

func (rl *RateLimiter) Worker() {
	for job := range rl.jobs {

		shard := rl.GetShard(job.key)
		if shard == nil {
			job.allow <- false
			rl.response <- Response{
				allowed: false,
				visitor: nil,
			}
			continue
		}

		shard.mu.RLock()
		visitor, ok := shard.visitor[job.key]
		if !ok {
			visitor = &visitors{
				visitorID: job.key,
			}
			shard.visitor[job.key] = visitor
		}
		shard.mu.RUnlock()

		if visitor.requestCount > 2 {
			job.allow <- false
			rl.response <- Response{
				allowed: false,
				visitor: visitor,
			}
			continue
		}
		visitor.requestCount++
		job.allow <- true

		rl.response <- Response{
			allowed: true,
			visitor: visitor,
		}
	}
}


func (rl *RateLimiter) CleanUp() {
	for _, shard := range rl.shards {
		shard.mu.Lock()
		for visitorID, visitor := range shard.visitor {
			if time.Since(visitor.lastVisit) > 70*time.Second {
				delete(shard.visitor, visitorID)
			}
		}
		shard.mu.Unlock()
	}
}
