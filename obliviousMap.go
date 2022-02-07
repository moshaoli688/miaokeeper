package main

import (
	"strconv"
	"time"

	"github.com/BBAlliance/miaokeeper/memutils"
)

type ObliviousMapIfce struct {
	prefix string
	driver memutils.MemDriver

	expire time.Duration
	utif   bool
}

func (om *ObliviousMapIfce) Get(key string) (interface{}, bool) {
	return om.driver.Read(om.prefix + key)
}

func (om *ObliviousMapIfce) Set(key string, value interface{}) interface{} {
	return om.driver.Write(om.prefix+key, value, om.expire, om.utif)
}

func (om *ObliviousMapIfce) Unset(key string) {
	om.driver.Expire(om.prefix + key)
}

func (om *ObliviousMapIfce) Exist(key string) bool {
	return om.driver.Exists(om.prefix + key)
}

type ObliviousMapInt struct {
	*ObliviousMapIfce
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
	return om.ObliviousMapIfce.Set(key, value).(int)
}

type ObliviousMapStr struct {
	*ObliviousMapIfce
}

func (om *ObliviousMapStr) Get(key string) (string, bool) {
	a, b := om.ObliviousMapIfce.Get(key)
	return a.(string), b
}

func (om *ObliviousMapStr) Set(key string, value string) string {
	return om.ObliviousMapIfce.Set(key, value).(string)
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
