package main

import (
	"net/url"
	"strconv"
	"strings"
	"sync"

	tb "gopkg.in/tucnak/telebot.v2"
)

type CallbackHandlerFn func(cp *CallbackParams)
type CallbackParams struct {
	url.Values

	Callback *tb.Callback
}

func (cp *CallbackParams) GetInt64(key string) (int64, bool) {
	if cp.Has(key) {
		if v, err := strconv.ParseInt(cp.Get(key), 10, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}

func (cp *CallbackParams) GetString(key string) (string, bool) {
	if cp.Has(key) {
		return cp.Get(key), true
	}
	return "", false
}

func (cp *CallbackParams) GetBool(key string) (bool, bool) {
	if cp.Has(key) {
		v := cp.Get(key)
		if ContainsString([]string{"true", "True", "TRUE", "yes", "y", "Y", "1"}, v) {
			return true, true
		}
		if ContainsString([]string{"false", "False", "FALSE", "no", "n", "N", "0"}, v) {
			return false, true
		}
	}
	return false, false
}

func (cp *CallbackParams) GetGroupId(key string) (int64, bool) {
	if v64, ok := cp.GetInt64(key); ok && v64 < 0 {
		return v64, true
	}
	return 0, false
}

func (cp *CallbackParams) GetUserId(key string) (int64, bool) {
	if v64, ok := cp.GetInt64(key); ok && v64 > 0 {
		return v64, true
	}
	return 0, false
}

func (cp *CallbackParams) GetValidGroup(key string) *GroupConfig {
	if groupId, ok := cp.GetGroupId(key); ok {
		if gc := GetGroupConfig(groupId); gc != nil {
			return gc
		}
	}
	return nil
}

func (cp *CallbackParams) GroupID() int64 {
	return cp.Callback.Message.Chat.ID
}

func (cp *CallbackParams) TriggerUser() *tb.User {
	return cp.Callback.Sender
}

func (cp *CallbackParams) TriggerUserID() int64 {
	return cp.Callback.Sender.ID
}

func (cp *CallbackParams) GroupConfig() *GroupConfig {
	return GetGroupConfig(cp.GroupID())
}

func (cp *CallbackParams) Locale() string {
	return GetSenderLocaleCallback(cp.Callback)
}

func (cp *CallbackParams) Response(payload string) {
	Rsp(cp.Callback, payload)
}

func (cp *CallbackParams) AssertType(key, vtype string) bool {
	ok := false
	if vtype == "bool" {
		_, ok = cp.GetBool(key)
	} else if vtype == "int64" {
		_, ok = cp.GetInt64(key)
	} else if vtype == "string" {
		_, ok = cp.GetString(key)
	} else if vtype == "group" {
		_, ok = cp.GetGroupId(key)
	} else if vtype == "user" {
		_, ok = cp.GetUserId(key)
	} else if vtype == "validgroup" {
		ok = cp.GetValidGroup(key) != nil
	}

	return ok
}

type CallbackHandlerConfig struct {
	CallbackFunction CallbackHandlerFn
	Route            string

	LockOn             string
	Validations        map[string]string
	ShouldInGroup      bool
	ShouldGroupAdmin   bool
	ShouldMiaoAdmin    bool
	ShouldMiaoAdminOpt string

	parent *CallbackHandler
}

func (chc *CallbackHandlerConfig) Call(cp *CallbackParams) {
	if chc.CallbackFunction != nil {
		if chc.LockOn != "" {
			chc.parent.Mutex[chc.LockOn].Lock()
			defer chc.parent.Mutex[chc.LockOn].Unlock()
		}
		chc.CallbackFunction(cp)
	}
}

func (chc *CallbackHandlerConfig) Should(key, vtype string) *CallbackHandlerConfig {
	chc.Validations[key] = vtype
	return chc
}

func (chc *CallbackHandlerConfig) Lock(key string) *CallbackHandlerConfig {
	if chc.parent.Mutex == nil {
		chc.parent.Mutex = make(map[string]*sync.Mutex)
	}
	if _, ok := chc.parent.Mutex[key]; !ok {
		chc.parent.Mutex[key] = &sync.Mutex{}
	}
	chc.LockOn = key
	return chc
}

func (chc *CallbackHandlerConfig) ShouldValidGroup(v bool) *CallbackHandlerConfig {
	chc.ShouldInGroup = v
	return chc
}

func (chc *CallbackHandlerConfig) ShouldValidMiaoAdminOpt(groupIdKey string) *CallbackHandlerConfig {
	chc.ShouldMiaoAdminOpt = groupIdKey
	return chc
}

func (chc *CallbackHandlerConfig) ShouldValidGroupAdmin(v bool) *CallbackHandlerConfig {
	chc.ShouldGroupAdmin = v
	return chc
}

func (chc *CallbackHandlerConfig) ShouldValidMiaoAdmin(v bool) *CallbackHandlerConfig {
	chc.ShouldMiaoAdmin = v
	return chc
}

func (chc *CallbackHandlerConfig) Match(c *tb.Callback) bool {
	return c.Data == chc.Route || strings.HasPrefix(c.Data, chc.Route+"?")
}

func (chc *CallbackHandlerConfig) Parse(c *tb.Callback) *CallbackParams {
	queryString := strings.Replace(c.Data, chc.Route+"?", "", 1)
	if chc.ShouldInGroup && GetGroupConfig(c.Message.Chat.ID) == nil {
		return nil
	}
	if chc.ShouldMiaoAdmin && !IsGroupAdminMiaoKo(c.Message.Chat, c.Sender) {
		return nil
	}
	if chc.ShouldGroupAdmin && !IsGroupAdmin(c.Message.Chat, c.Sender) {
		return nil
	}
	if u, err := url.ParseQuery(queryString); err == nil && u != nil {
		qs := CallbackParams{u, c}
		if chc.ShouldMiaoAdminOpt != "" {
			gid, _ := qs.GetGroupId(chc.ShouldMiaoAdminOpt)
			if !IsGroupAdminMiaoKo(&tb.Chat{ID: gid}, c.Sender) {
				return nil
			}
		}
		for key, vType := range chc.Validations {
			if !qs.AssertType(key, vType) {
				return nil
			}
		}
		return &qs
	} else {
		return nil
	}
}

type CallbackHandler struct {
	Routes map[string]*CallbackHandlerConfig
	Mutex  map[string]*sync.Mutex
}

func (ch *CallbackHandler) Add(route string, fn CallbackHandlerFn) *CallbackHandlerConfig {
	chc := &CallbackHandlerConfig{
		Route:            route,
		CallbackFunction: fn,
		Validations:      make(map[string]string),

		parent: ch,
	}

	if ch.Routes == nil {
		ch.Routes = make(map[string]*CallbackHandlerConfig)
	}
	ch.Routes[route] = chc
	return chc
}

func (ch *CallbackHandler) Handle(c *tb.Callback) {
	c.Data = strings.TrimSpace(c.Data)
	DLogf("Callback Event | group=%d user=%d data=%s", c.Message.Chat.ID, c.Sender.ID, c.Data)
	for _, r := range ch.Routes {
		if r != nil && r.CallbackFunction != nil && r.Match(c) {
			if cp := r.Parse(c); cp != nil {
				r.Call(cp)
			} else {
				// validation failed
				Rsp(c, "cb.validationError")
			}
			return
		}
	}

	// no route match, invalid request
	Rsp(c, "cb.notParsed")
}
