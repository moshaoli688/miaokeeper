package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func AddIntoInt64Arr(arr []int64, val int64) ([]int64, bool) {
	res := []int64{}
	seen := false
	for _, s := range arr {
		res = append(res, s)
		if s == val {
			seen = true
		}
	}
	if !seen {
		res = append(res, val)
	}
	return res, !seen
}

func DelFromInt64Arr(arr []int64, val int64) ([]int64, bool) {
	res := []int64{}
	seen := false
	for _, s := range arr {
		if s != val {
			res = append(res, s)
		} else {
			seen = true
		}
	}
	return res, seen
}

func ParseInt64ArrToStr(arr []int64) string {
	res := []string{}
	for _, s := range arr {
		res = append(res, strconv.FormatInt(s, 10))
	}
	return strings.Join(res, ",")
}

func ParseStrToInt64Arr(str string) []int64 {
	res := []int64{}
	resStr := strings.Split(str, ",")
	for _, s := range resStr {
		i, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			res = append(res, i)
		}
	}

	return res
}

func I64In(arr *[]int64, target int64) bool {
	for _, i := range *arr {
		if i == target {
			return true
		}
	}
	return false
}

func MakeSysChan() chan os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return sigCh
}

func MinInt(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func MaxInt(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func MinInt64(a, b int64) int64 {
	if a > b {
		return b
	}
	return a
}

func MaxInt64(a, b int64) int64 {
	if a < b {
		return b
	}
	return a
}

func Abs(a int64) int64 {
	if a >= 0 {
		return a
	}
	return -a
}

func ParseInt64(s string) int64 {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return 0
}

func Type(to interface{}) string {
	if to == nil {
		return "nil"
	}
	return reflect.TypeOf(to).String()
}

func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func MD5(str string) string {
	hasher := md5.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}

func SignGroup(groupId int64, signType int, seed string) string {
	phaseOne := MD5(fmt.Sprintf("MiaoKeeper:Normal:Hash|%d%d|%s|APITK{%d}##^", signType, groupId, APISeed, groupId))
	phaseTwo := MD5(fmt.Sprintf("501%s5c%dadd%s51f%sf13%d", phaseOne, signType, phaseOne, seed, signType))
	phaseThree := MD5(fmt.Sprintf("%s415%s%s%daff4%s", phaseOne, seed, phaseTwo, signType, phaseOne))

	return phaseThree
}

func POSTJsonWithSign(url string, sign string, payload []byte, timeout time.Duration) []byte {
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil
	}

	req.Header.Set("User-Agent", "miaokeeper/"+version)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MiaoKeeper-Sign", sign)
	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)

	if err != nil {
		return nil
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	return body
}

func WarpError(fn func()) (err error) {
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
	}()

	fn()
	return
}

func PlainError(info string, err error) {
	if err != nil {
		DErrorf("Unexpected Error | %s error=%v", info, err.Error())
	}
}

func SetInterval(interval time.Duration, fn func()) func() {
	ticker := time.NewTicker(interval)
	quit := make(chan bool)
	active := true

	go func() {
		for {
			select {
			case <-ticker.C:
				go fn()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	// canceller
	return func() {
		if active {
			active = false
			quit <- true
		}
	}
}

func Throttle(interval time.Duration) (func(func()), func()) {
	var adaptor func() = nil
	updateLock := sync.Mutex{}

	cancel := SetInterval(interval, func() {
		updateLock.Lock()
		defer updateLock.Unlock()

		if adaptor != nil {
			go adaptor()
		}
	})

	return func(fn func()) {
		updateLock.Lock()
		defer updateLock.Unlock()

		adaptor = fn
	}, cancel
}
