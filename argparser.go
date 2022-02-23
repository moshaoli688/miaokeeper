package main

import (
	"reflect"
	"strconv"
	"strings"
)

type ArgHolder struct {
	storage map[string]interface{}
}

func (ah *ArgHolder) Parse(payload string) string {
	ah.storage = make(map[string]interface{})
	args := strings.Fields(payload)
	keptArgs := []string{}
	for _, arg := range args {
		kv := strings.SplitN(arg, "=", 2)
		if len(kv) == 2 && strings.HasPrefix(kv[0], ":") {
			k := kv[0][1:]
			if strings.Contains("y Y yes YES true TRUE", kv[1]) {
				ah.storage[k] = true
			} else if strings.Contains("n N no NO false FALSE", kv[1]) {
				ah.storage[k] = false
			} else if i, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
				ah.storage[k] = i
			} else {
				ah.storage[k] = kv[1]
			}
		} else {
			keptArgs = append(keptArgs, arg)
		}
	}
	return strings.TrimSpace(strings.Join(keptArgs, " "))
}

func (ah *ArgHolder) Int64(key string) (int64, bool) {
	if Type(ah.storage[key]) == reflect.Int64.String() {
		return ah.storage[key].(int64), true
	}
	return 0, false
}

func (ah *ArgHolder) Int(key string) (int, bool) {
	if v64, ok := ah.Int64(key); ok {
		return int(v64), true
	}
	return 0, false
}

func (ah *ArgHolder) Bool(key string) (bool, bool) {
	if Type(ah.storage[key]) == reflect.Bool.String() {
		return ah.storage[key].(bool), true
	}
	return false, false
}

func (ah *ArgHolder) Str(key string) (string, bool) {
	if Type(ah.storage[key]) == reflect.String.String() {
		return ah.storage[key].(string), true
	}
	return "", false
}

func ArgParse(payload string) (string, *ArgHolder) {
	ah := &ArgHolder{}
	return ah.Parse(payload), ah
}
