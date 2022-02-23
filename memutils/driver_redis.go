package memutils

import (
	"context"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

type MemDriverRedis struct {
	MemDriver

	rdb *redis.Client
	ctx context.Context
}

func (md *MemDriverRedis) Init(kargs ...string) {
	if len(kargs) < 2 {
		Log(os.Stdout, "MemDriver Error | Should have two parameters to indicate host and password")
		os.Exit(1)
	}

	md.ctx = context.Background()
	md.rdb = redis.NewClient(&redis.Options{
		Addr:     kargs[0],
		Password: kargs[1],
		DB:       0,
	})

	if err := md.rdb.Ping(md.ctx).Err(); err != nil {
		Log(os.Stdout, "MemDriver Error | Cannot ping server: "+err.Error())
		os.Exit(1)
	}
}

func (md *MemDriverRedis) Read(key string) (interface{}, bool) {
	val, err := md.rdb.Get(md.ctx, key).Result()
	return val, err == nil
}

func (md *MemDriverRedis) Write(key string, value interface{}, expire time.Duration, overwriteTTLIfExists bool) interface{} {
	if !overwriteTTLIfExists {
		if duration, err := md.rdb.TTL(md.ctx, key).Result(); err == nil {
			expire = duration
		}
	}
	md.rdb.Set(md.ctx, key, value, expire)

	return value
}

func (md *MemDriverRedis) Inc(key string, expire time.Duration, overwriteTTLIfExists bool) int {
	if !overwriteTTLIfExists {
		if duration, err := md.rdb.TTL(md.ctx, key).Result(); err == nil && duration > time.Second {
			expire = duration
		}
	}
	ret := md.rdb.Incr(md.ctx, key).Val()
	md.rdb.Expire(md.ctx, key, expire)

	return int(ret)
}

func (md *MemDriverRedis) Expire(key string) {
	md.rdb.Unlink(md.ctx, key)
}

func (md *MemDriverRedis) Exists(key string) bool {
	return md.rdb.Exists(md.ctx, key).Val() == 1
}

func (md *MemDriverRedis) List(prefix string) []string {
	return md.rdb.Keys(md.ctx, prefix+"*").Val()
}
