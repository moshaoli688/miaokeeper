package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/BBAlliance/miaokeeper/memutils"
	"github.com/bep/debounce"
	tb "gopkg.in/telebot.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORM:%NAME%_Credit_%GROUP%
type CreditInfoSkeleton struct {
	ID       int64  `json:"id" gorm:"column:userid;primaryKey;not null"`
	Username string `json:"username" gorm:"column:username;type:text;not null"`
	Name     string `json:"nickname" gorm:"column:name;type:text;not null"`
	Credit   int64  `json:"credit" gorm:"column:credit;not null"`
	GroupId  int64  `json:"groupId" gorm:"-"`
}

type CreditInfo struct {
	CreditInfoSkeleton

	updateLock      sync.Mutex   `json:"-" gorm:"-"`
	updateDebouncer func(func()) `json:"-" gorm:"-"`
}

type UserInfo struct {
	Group    int64
	ID       int64
	Username string
	Name     string
}

func (ui *UserInfo) From(groupId int64, user *tb.User) *UserInfo {
	ui.ID = user.ID
	ui.Name = GetQuotableUserName(user)
	ui.Username = user.Username
	ui.Group = groupId
	return ui
}

var CreditInfoCache *ObliviousMapIfce

func GetCreditRank(groupId int64, limit int) []*CreditInfoSkeleton {
	returns := []*CreditInfoSkeleton{}
	realGroup := GetAliasedGroup(groupId)
	DB.Table(DBTName("Credit", realGroup)).Order("credit DESC").Limit(limit).Find(&returns)
	for _, ci := range returns {
		if ci != nil && ci.ID > 0 {
			ci.GroupId = groupId
		}
	}
	return returns
}

// does not apply MergeTo
func DumpCredits(groupId int64) [][]string {
	ret := [][]string{}
	batches := []CreditInfoSkeleton{}
	DB.Table(DBTName("Credit", groupId)).FindInBatches(&batches, 100, func(tx *gorm.DB, batchNum int) error {
		for _, batch := range batches {
			if batch.ID > 0 && batch.Credit > 0 {
				ret = append(ret, []string{strconv.FormatInt(batch.ID, 10), batch.Name, batch.Username, strconv.FormatInt(batch.Credit, 10)})
			}
		}
		return nil
	})

	DInfof("Credit Dump | group=%d columns=%d", groupId, len(ret))
	return ret
}

// does not apply MergeTo
func FlushCredits(groupId int64, records [][]string, executor int64) {
	if len(records) == 0 {
		return
	}

	creditInfoWritingLock.Lock()
	defer creditInfoWritingLock.Unlock()

	// 清除所有 CreditInfoCache 防止脏写入
	// 但对于已经实例化的 CreditInfo 来说还是可能存在刷写不一致问题
	// 这里采用等待 100ms 的方法，使 debouncer清空。虽然依旧存在问题，但微乎其微
	time.Sleep(time.Millisecond * 100)
	CreditInfoCache.Wipe()

	batches := []CreditInfoSkeleton{}
	logbatches := []CreditLog{}
	for _, r := range records {
		if len(r) >= 4 {
			batches = append(batches, CreditInfoSkeleton{
				ID:       ParseInt64(r[0]),
				Name:     r[1],
				Username: r[2],
				Credit:   ParseInt64(r[3]),
			})
			logbatches = append(logbatches, CreditLog{
				UserID:   ParseInt64(r[0]),
				Credit:   ParseInt64(r[3]),
				Reason:   OPFlush,
				Executor: executor,
			})
		}
	}
	err := DB.Table(DBTName("Credit", groupId)).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "userid"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "username", "credit"}),
	}).CreateInBatches(&batches, 100).Error
	if err != nil {
		DErrorE(err, "Database Credit Flush Error")
	}

	// writing logs
	go PushCreditLogs(groupId, logbatches...)
	DInfof("Flush Credit | group=%d columns=%d", groupId, len(records))
}

func (ci *CreditInfo) Acquire(fn func()) {
	ci.updateLock.Lock()
	defer ci.updateLock.Unlock()

	fn()
}

func (ci *CreditInfo) unsafeSync() {
	if ci.updateDebouncer == nil {
		ci.updateDebouncer = debounce.New(time.Second)
	}

	ci.updateDebouncer(func() {
		if err := DB.Table(DBTName("Credit", ci.GroupId)).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "userid"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "username", "credit"}),
		}).Create(&ci).Error; err != nil {
			DErrorE(err, "Database Credit Update Error")
		}
		DLogf("Credit Update Debouncer | gid=%d uid=%d credit=%d", ci.GroupId, ci.ID, ci.Credit)
	})
}

func (ci *CreditInfo) unsafeUpdate(method UpdateMethod, value int64, ui *UserInfo, reason OPReasons, executor int64, notes string) *CreditInfo {
	if ci == nil || ci.ID <= 0 {
		return nil
	}

	// update user info
	if ui != nil {
		ci.Name = ui.Name
		ci.Username = ui.Username
	}

	// prepare log
	logCredit := value
	if method == UMDel {
		logCredit = -ci.Credit
	}

	// update credit
	if method == UMSet {
		ci.Credit = value
	} else if method == UMAdd {
		ci.Credit += value
	} else if method == UMDel {
		ci.Credit = 0
	}

	// flush log
	go PushCreditLogs(ci.GroupId, CreditLog{
		UserID:   ci.ID,
		Credit:   logCredit,
		Reason:   reason,
		Executor: executor,
		Notes:    notes,
	})

	DLogf("Update Credit | gid=%d user=%d alter=%d credit=%d", ci.GroupId, ci.ID, method, value)
	ci.unsafeSync()
	return ci
}

func (ci *CreditInfo) Update(method UpdateMethod, value int64, ui *UserInfo, reason OPReasons, executor int64, notes string) *CreditInfo {
	if ci == nil {
		return nil
	}

	ci.updateLock.Lock()
	defer ci.updateLock.Unlock()
	return ci.unsafeUpdate(method, value, ui, reason, executor, notes)
}

var creditInfoWritingLock sync.Mutex

func GetCreditInfo(groupId, userId int64) *CreditInfo {
	groupId = GetAliasedGroup(groupId)
	cicKey := fmt.Sprintf("%d-%d", groupId, userId)
	if cii, ok := CreditInfoCache.Get(cicKey); ok && cii != nil {
		if ci, ok := cii.(*CreditInfo); ok && ci != nil {
			return ci
		}
	}

	creditInfoWritingLock.Lock()
	defer creditInfoWritingLock.Unlock()

	ret := &CreditInfo{}
	err := DB.Table(DBTName("Credit", groupId)).First(&ret, "userid = ?", userId).Error
	if err != nil {
		if err.Error() == "record not found" {
			// if is new profile, just create and write to cache
			ret.ID = userId
			CreditInfoCache.Set(cicKey, ret)
			DLogf("Database Credit Create New Profile | gid=%d uid=%d", groupId, userId)
		} else {
			DLogf("Database Credit Read Error | gid=%d uid=%d error=%s", groupId, userId, err.Error())
		}
	}

	if ret.ID == userId {
		ret.GroupId = groupId
		CreditInfoCache.Set(cicKey, ret)
	}
	return ret
}

func UpdateCredit(ui *UserInfo, method UpdateMethod, value int64, reason OPReasons, executor int64, notes string) *CreditInfo {
	ci := GetCreditInfo(ui.Group, ui.ID)
	return ci.Update(method, value, ui, reason, executor, notes)
}

func init() {
	memdriver := &memutils.MemDriverMemory{}
	memdriver.Init()
	CreditInfoCache = NewOMapIfce("cicache/", time.Hour, true, memdriver)
}
