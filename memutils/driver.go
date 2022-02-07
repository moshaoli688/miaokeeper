package memutils

import "time"

type MemDriver interface {
	Init(kargs ...string)

	Read(key string) (interface{}, bool)
	Write(key string, value interface{}, expire time.Duration, overwriteTTLIfExists bool) interface{}
	Inc(key string, expire time.Duration, overwriteTTLIfExists bool) int

	List(prefix string) []string
	Expire(key string)
	Exists(key string) bool
}

func Now() int64 {
	return time.Now().UnixNano()
}
