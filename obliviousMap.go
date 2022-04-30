package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/BBAlliance/miaokeeper/memutils"
)

type ObliviousMapIfce struct {
	prefix string
	driver memutils.MemDriver

	expire time.Duration
	hold   sync.Mutex
	utif   bool
}

func (om *ObliviousMapIfce) Hold(fn func()) {
	om.hold.Lock()
	defer om.hold.Unlock()

	fn()
}

func (om *ObliviousMapIfce) Get(key string) (interface{}, bool) {
	return om.driver.Read(om.prefix + key)
}

func (om *ObliviousMapIfce) Set(key string, value interface{}) interface{} {
	return om.driver.Write(om.prefix+key, value, om.expire, om.utif)
}

func (om *ObliviousMapIfce) SetExpire(key string, duration time.Duration) time.Duration {
	return om.driver.SetExpire(om.prefix+key, duration)
}

func (om *ObliviousMapIfce) Unset(key string) {
	om.driver.Expire(om.prefix + key)
}

func (om *ObliviousMapIfce) Exist(key string) bool {
	return om.driver.Exists(om.prefix + key)
}

func (om *ObliviousMapIfce) Wipe() {
	om.driver.Wipe(om.prefix)
}

func (om *ObliviousMapIfce) WipePrefix(prefix string) {
	om.driver.WipePrefix(om.prefix + prefix)
}

type ObliviousMapInt struct {
	*ObliviousMapIfce
}

func (om *ObliviousMapInt) AddBy(key string, val int) int {
	return om.driver.IncBy(om.prefix+key, val, om.expire, om.utif)
}

func (om *ObliviousMapInt) Add(key string) int {
	return om.driver.Inc(om.prefix+key, om.expire, om.utif)
}

func (om *ObliviousMapInt) Get(key string) (int, bool) {
	a, b := om.ObliviousMapIfce.Get(key)
	if a == nil {
		return 0, b
	}
	switch x := a.(type) {
	case string:
		v, err := strconv.Atoi(x)
		return v, err == nil
	case int64:
		return int(x), b
	case int:
		return x, b
	default:
		return 0, false
	}
}

func (om *ObliviousMapInt) Set(key string, value int) int {
	val := om.ObliviousMapIfce.Set(key, value)
	if val == nil {
		return 0
	}
	return val.(int)
}

type ObliviousMapStr struct {
	*ObliviousMapIfce
}

func (om *ObliviousMapStr) Get(key string) (string, bool) {
	a, b := om.ObliviousMapIfce.Get(key)
	if a == nil {
		return "", false
	}
	return a.(string), b
}

func (om *ObliviousMapStr) Set(key string, value string) string {
	val := om.ObliviousMapIfce.Set(key, value)
	if val == nil {
		return ""
	}
	return val.(string)
}

func NewOMapIfce(prefix string, expire time.Duration, updateTimeIfWrite bool, driver memutils.MemDriver) *ObliviousMapIfce {
	return &ObliviousMapIfce{
		prefix: prefix,
		expire: expire,
		driver: driver,
		utif:   updateTimeIfWrite,
	}
}

func NewOMapInt(prefix string, duration time.Duration, updateTimeIfWrite bool, driver memutils.MemDriver) *ObliviousMapInt {
	return &ObliviousMapInt{
		ObliviousMapIfce: NewOMapIfce(prefix, duration, updateTimeIfWrite, driver),
	}
}

func NewOMapStr(prefix string, duration time.Duration, updateTimeIfWrite bool, driver memutils.MemDriver) *ObliviousMapStr {
	return &ObliviousMapStr{
		ObliviousMapIfce: NewOMapIfce(prefix, duration, updateTimeIfWrite, driver),
	}
}
