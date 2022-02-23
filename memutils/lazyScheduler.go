package memutils

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"time"

	jsoniter "github.com/json-iterator/go"
)

type LazySchedulerHandler = func(*LazySchedulerCall)
type LazyScheduler struct {
	driver MemDriver
	fn     map[string]LazySchedulerHandler
}

func NewLazyScheduler(driver MemDriver) *LazyScheduler {
	return &LazyScheduler{
		driver: driver,
		fn:     map[string]LazySchedulerHandler{},
	}
}

type LazySchedulerCall struct {
	ID   string
	Name string
	At   int64
	Args string
}

func (lsc *LazySchedulerCall) Arg(to interface{}) {
	if lsc.Args != "" {
		jsoniter.UnmarshalFromString(lsc.Args, to)
	}
}

func LSC(fnName string, args interface{}) *LazySchedulerCall {
	argStr, _ := jsoniter.MarshalToString(args)
	return &LazySchedulerCall{
		Name: fnName,
		Args: argStr,
	}
}

func (ls *LazyScheduler) Reg(fnName string, handler LazySchedulerHandler) {
	if ls != nil {
		ls.fn[fnName] = handler
	}
}

func (ls *LazyScheduler) GenerateKey(timestamp int64) string {
	return fmt.Sprintf("timer/%d/%d", timestamp, rand.Intn(10000))
}

func (ls *LazyScheduler) Recover() {
	future := ls.driver.List("timer/")
	counter := 0
	for _, key := range future {
		if s, ok := ls.driver.Read(key); ok && s != nil && reflect.TypeOf(s).Kind() == reflect.String {
			call := &LazySchedulerCall{}
			jsoniter.UnmarshalFromString(s.(string), call)
			time.AfterFunc(time.Duration(call.At-Now()), func() {
				ls.Exec(call)
			})
			counter += 1
		}
	}
	Log(os.Stdout, fmt.Sprintf("System | Recovered %d tasks from cache\n", counter))
}

func (ls *LazyScheduler) After(duration time.Duration, call *LazySchedulerCall) {
	if ls != nil {
		call.At = Now() + duration.Nanoseconds()
		call.ID = ls.GenerateKey(call.At)
		caller, _ := jsoniter.MarshalToString(call)
		ls.driver.Write(call.ID, caller, time.Hour*24*30, true)
		time.AfterFunc(duration, func() {
			ls.Exec(call)
		})
	}
}

func (ls *LazyScheduler) Exec(call *LazySchedulerCall) (err error) {
	defer func() {
		if erro := recover(); erro != nil {
			switch x := erro.(type) {
			case string:
				err = fmt.Errorf(x)
			case error:
				err = x
			default:
				err = fmt.Errorf("unknown error")
			}
		}

		// remove task
		ls.driver.Expire(call.ID)
	}()

	if ls != nil && call != nil {
		if fn, ok := ls.fn[call.Name]; ok && fn != nil {
			fn(call)
		}
	}

	return
}
