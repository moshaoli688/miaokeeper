package main

import (
	"fmt"
	"strings"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	jsoniter "github.com/json-iterator/go"
)

var DBCONN = ""
var DBPREFIX = "MiaoKeeper"
var DB *gorm.DB
var UpdateLock sync.Mutex
var GroupConfigLock sync.Mutex

type UpdateMethod int

const (
	UMSet UpdateMethod = iota
	UMAdd
	UMDel
)

func DBTName(tableName string, extras ...int64) string {
	table := DBPREFIX + "_" + tableName
	if len(extras) > 0 {
		table += "_" + fmt.Sprintf("%d", Abs(extras[0]))
	}
	return table
}

func GetRandClause() string {
	if strings.HasPrefix(DBCONN, "postgres") {
		return "RANDOM()"
	}
	return "RAND()"
}

// GORM:%NAME%_Config
type DBGlobalConfig struct {
	K string `gorm:"column:k;type:string;primaryKey;size:128;not null"`
	V string `gorm:"column:v;type:text;not null"`
}

func InitDatabase() (err error) {
	var conn gorm.Dialector
	if strings.HasPrefix(DBCONN, "postgres") {
		// postgresSQL
		conn = postgres.Open(DBCONN)
	} else {
		// mysql
		connStr := DBCONN
		if strings.Contains(connStr, "?") {
			connStr += "&parseTime=true"
		} else {
			connStr += "?parseTime=true"
		}
		conn = mysql.Open(connStr)
	}

	DB, err = gorm.Open(conn, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	return
}

func ReadConfig(key string) string {
	dbgc := DBGlobalConfig{}
	err := DB.Table(DBTName("Config")).First(&dbgc, "k = ?", key).Error
	if err != nil {
		DLogf("Config Read Error | key=%s error=%v", key, err.Error())
	}
	return dbgc.V
}

func WriteConfig(key, value string) {
	dbgc := DBGlobalConfig{key, value}
	err := DB.Table(DBTName("Config")).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "k"}},
		DoUpdates: clause.AssignmentColumns([]string{"v"}),
	}).Create(&dbgc).Error
	if err != nil {
		DLogf("Config Write Error | key=%s value=%s error=%v", key, value, err.Error())
	}
}

func GetAliasedGroup(groupId int64) int64 {
	if gc := GetGroupConfig(groupId); gc != nil {
		if gc.MergeTo < 0 {
			return gc.MergeTo
		}
	}
	return groupId
}

var GroupConfigCache map[int64]*GroupConfig

func GetGroupConfig(groupId int64) *GroupConfig {
	if !IsGroup(groupId) {
		return nil
	}

	GroupConfigLock.Lock()
	defer GroupConfigLock.Unlock()
	if gc, ok := GroupConfigCache[groupId]; ok {
		return gc
	}

	cfg := ReadConfig(fmt.Sprintf("gcfg_%d", Abs(groupId)))
	if cfg != "" {
		gc := &GroupConfig{}
		err := jsoniter.Unmarshal([]byte(cfg), gc)
		if err == nil {
			GroupConfigCache[groupId] = gc.Check()
			return gc
		}
	}
	return nil
}

func SetGroupConfig(groupId int64, gc *GroupConfig) *GroupConfig {
	if !IsGroup(groupId) {
		return nil
	}

	if groupId >= 0 {
		return nil
	}

	GroupConfigLock.Lock()
	defer GroupConfigLock.Unlock()
	GroupConfigCache[groupId] = gc

	cfg, _ := jsoniter.Marshal(gc)
	if len(cfg) > 0 {
		WriteConfig(fmt.Sprintf("gcfg_%d", Abs(groupId)), string(cfg))
	}
	return gc
}

func InitGroupTable(groupId int64) {
	PlainError("Unable to create table", DB.Table(DBTName("Credit", groupId)).AutoMigrate(&CreditInfo{}))
	PlainError("Unable to create table", DB.Table(DBTName("Credit_Log", groupId)).AutoMigrate(&CreditLog{}))

	if GetGroupConfig(groupId) == nil {
		NewGroupConfig(groupId)
	}
}

func ReadConfigs() {
	PlainError("Unable to create config table", DB.Table(DBTName("Config")).AutoMigrate(&DBGlobalConfig{}))
	PlainError("Unable to create lottery table", DB.Table(DBTName("Lottery")).AutoMigrate(&DBLottery{}))
	PlainError("Unable to create table", DB.Table(DBTName("Lottery_Participation")).AutoMigrate(&DBLotteryParticipation{}))

	ADMINS = ParseStrToInt64Arr(ReadConfig("ADMINS"))
	GROUPS = ParseStrToInt64Arr(ReadConfig("GROUPS"))

	for _, g := range GROUPS {
		InitGroupTable(g)
	}
}

func WriteConfigs() {
	WriteConfig("ADMINS", ParseInt64ArrToStr(ADMINS))
	WriteConfig("GROUPS", ParseInt64ArrToStr(GROUPS))
}

func UpdateAdmin(userId int64, method UpdateMethod) bool {
	UpdateLock.Lock()
	defer UpdateLock.Unlock()
	changed := false
	if method == UMSet {
		if len(ADMINS) != 1 || ADMINS[0] != userId {
			changed = true
			ADMINS = []int64{userId}
		}
	} else if method == UMAdd {
		ADMINS, changed = AddIntoInt64Arr(ADMINS, userId)
	} else if method == UMDel {
		ADMINS, changed = DelFromInt64Arr(ADMINS, userId)
	}
	WriteConfigs()
	return changed
}

func UpdateGroup(groupId int64, method UpdateMethod) bool {
	UpdateLock.Lock()
	defer UpdateLock.Unlock()
	changed := false
	if method == UMSet {
		if len(GROUPS) != 1 || GROUPS[0] != groupId {
			changed = true
			GROUPS = []int64{groupId}
		}
	} else if method == UMAdd {
		GROUPS, changed = AddIntoInt64Arr(GROUPS, groupId)
	} else if method == UMDel {
		GROUPS, changed = DelFromInt64Arr(GROUPS, groupId)
	}
	if changed && method == UMSet || method == UMAdd {
		InitGroupTable(groupId)
	}
	WriteConfigs()
	return changed
}

func init() {
	GroupConfigCache = make(map[int64]*GroupConfig)
}
