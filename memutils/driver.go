package memutils

import "time"

type MemDriver interface {
	Init(kargs ...string)

	Read(key string) (interface{}, bool)
	Write(key string, value interface{}, expire time.Duration, overwriteTTLIfExists bool) interface{}
	IncBy(key string, value int, expire time.Duration, overwriteTTLIfExists bool) int
	Inc(key string, expire time.Duration, overwriteTTLIfExists bool) int

	List(prefix string) []string
	Expire(key string)
	Exists(key string) bool
	Wipe(prefix string)
}

func Now() int64 {
	return time.Now().UnixNano()
}
