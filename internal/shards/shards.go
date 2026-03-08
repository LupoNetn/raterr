package shards

import (
	"hash/fnv"
	"sync"
	"time"
)

type Shard struct {
	mu sync.RWMutex
	visitor map[string]*visitors
}

type visitors struct {
   visitorID string
   requestCount int
   lastVisit time.Time
}

type RateLimiter struct {
	shards []*Shard
	shardCount int
	
}


type Job struct {
	key string
	allow chan bool
}

func NewRateLimiter(shardCount int) *RateLimiter {
	shardSlice := make([]*Shard, shardCount)
	for i := 0; i < shardCount; i++ {
		shardSlice[i] = &Shard{
			visitor: make(map[string]*visitors),
		}
	}

	return &RateLimiter{
		shards: shardSlice,
		shardCount: shardCount,
	}
}


//rate limiter functions 
func (rl *RateLimiter) GetShard(key string) *Shard {

	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	index := int(hasher.Sum32() % uint32(rl.shardCount))

	shard := rl.shards[index]
	if shard == nil {
		panic("shard not found")
	}
	return shard
}

