package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type LogType int

var VerboseMode bool

const (
	LTLog LogType = iota
	LTInfo
	LTWarn
	LTError
)

type LogUnit struct {
	Type    LogType
	Data    string
	Time    int64
	TimeStr string
}

func (l *LogUnit) Error() error {
	return errors.New(l.Data)
}

var LogBank []*LogUnit
var LogBankMutex sync.RWMutex

func LogTypeToStr(lt LogType) string {
	if lt == LTLog {
		return "ALOG"
	} else if lt == LTInfo {
		return "INFO"
	} else if lt == LTWarn {
		return "WARN"
	} else if lt == LTError {
		return "ERRO"
	}
	return "UDEF"
}

func PrintLogUnit(lu *LogUnit) {
	if VerboseMode || (lu.Type == LTLog || lu.Type == LTError) {
		if lu.Type <= LTWarn {
			fmt.Fprintf(os.Stdout, "%s | %s | %s", LogTypeToStr(lu.Type), lu.TimeStr, lu.Data)
		} else {
			fmt.Fprintf(os.Stdout, "%s | %s | %s", LogTypeToStr(lu.Type), lu.TimeStr, lu.Data)
		}
	}
}

func PushLogUnit(lu *LogUnit) int {
	LogBankMutex.Lock()
	defer LogBankMutex.Unlock()
	if LogBank == nil {
		LogBank = make([]*LogUnit, 0)
	}
	LogBank = append(LogBank, lu)
	return len(LogBank)
}

func GetLogBank(left, length int) []*LogUnit {
	LogBankMutex.RLock()
	defer LogBankMutex.RUnlock()
	if LogBank == nil {
		return []*LogUnit{}
	}
	right := left + length
	left = MaxInt(left, 0)
	right = MaxInt(right, 0)
	left = MinInt(left, len(LogBank))
	right = MinInt(right, len(LogBank))
	right = MaxInt(right, left)
	return LogBank[left:right]
}

func GetLastLogBank(toRight, length int) []*LogUnit {
	return GetLogBank(len(LogBank)-toRight-length, length)
}

func DBase(t LogType, a ...interface{}) *LogUnit {
	currentTime := time.Now()
	data := fmt.Sprintln(a...)
	log := LogUnit{
		Time:    currentTime.UnixNano(),
		TimeStr: currentTime.Format(time.RFC3339),
		Data:    data,
		Type:    t,
	}
	PushLogUnit(&log)
	PrintLogUnit(&log)
	return &log
}

func DBasef(t LogType, format string, a ...interface{}) *LogUnit {
	return DBase(t, fmt.Sprintf(format, a...))
}

func DLog(a ...interface{}) *LogUnit {
	return DBase(LTLog, a...)
}

func DLogf(format string, a ...interface{}) *LogUnit {
	return DBasef(LTLog, format, a...)
}

func DInfo(a ...interface{}) *LogUnit {
	return DBase(LTInfo, a...)
}

func DInfof(format string, a ...interface{}) *LogUnit {
	return DBasef(LTInfo, format, a...)
}

func DWarn(a ...interface{}) *LogUnit {
	return DBase(LTWarn, a...)
}

func DWarnf(format string, a ...interface{}) *LogUnit {
	return DBasef(LTWarn, format, a...)
}

func DError(a ...interface{}) *LogUnit {
	return DBase(LTError, a...)
}

func DErrorf(format string, a ...interface{}) *LogUnit {
	return DBasef(LTError, format, a...)
}

func DErrorE(err error, a ...interface{}) *LogUnit {
	if err != nil {
		a = append(a, err.Error())
	}
	return DBase(LTError, a...)
}

func DErrorEf(err error, format string, a ...interface{}) *LogUnit {
	if err != nil {
		format = strings.TrimSpace(format) + " | error=%s"
		a = append(a, err.Error())
	}
	return DBasef(LTError, format, a...)
}
