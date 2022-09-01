package main

import (
	"sync"
	"time"
)

// GORM:%NAME%_Credit_Log_%GROUP%
type CreditLog struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement;not null"`
	UserID    int64     `gorm:"column:userid;not null;index"`
	Credit    int64     `gorm:"column:credit;not null"`
	Reason    OPReasons `gorm:"column:op;type:string;size:16;not null;index"`
	Executor  int64     `gorm:"column:executor;index"`
	Notes     string    `gorm:"column:notes;type:string;size:64"`
	CreatedAt time.Time `gorm:"column:createdat;autoCreateTime"`
}

type OPReasons string

const (
	OPAll              OPReasons = ""
	OPFlush            OPReasons = "FLUSH"
	OPNormal           OPReasons = "NORMAL"
	OPByAdmin          OPReasons = "ADMIN"
	OPByAdminSet       OPReasons = "ADMINSET"
	OPByRedPacket      OPReasons = "REDPACKET"
	OPByLottery        OPReasons = "LOTTERY"
	OPByLotteryReverse OPReasons = "LOTTERY_REV"
	OPByTransfer       OPReasons = "TRANSFER"
	OPByPolicy         OPReasons = "POLICY"
	OPByAbuse          OPReasons = "ABUSE"
	OPByAPIConsume     OPReasons = "CONSUME"
	OPByAPIBonus       OPReasons = "BONUS"
	OPByCleanUp        OPReasons = "CLEANUP"
)

var OPAllReasons = []OPReasons{OPAll, OPFlush, OPNormal, OPByAdmin, OPByAdminSet, OPByRedPacket, OPByLottery, OPByLotteryReverse, OPByTransfer, OPByPolicy, OPByAbuse, OPByAPIConsume, OPByCleanUp}

func (op *OPReasons) Repr() string {
	if *op == OPAll {
		return "ALL"
	} else {
		return string(*op)
	}
}

func OPParse(s string) OPReasons {
	for _, op := range OPAllReasons {
		if string(op) == s {
			return op
		}
	}

	return OPAll
}

type CreditLogBank struct {
	Group      int64
	Logs       []CreditLog
	updateLock sync.Mutex
}

var CreditLogBanks map[int64]*CreditLogBank
var CreditLogBankLock sync.Mutex

func (clb *CreditLogBank) Flush() {
	clb.updateLock.Lock()
	logbatches := clb.Logs
	clb.Logs = make([]CreditLog, 0)
	clb.updateLock.Unlock()

	if len(logbatches) == 0 {
		return
	}

	err := DB.Table(DBTName("Credit_Log", clb.Group)).CreateInBatches(&logbatches, 200).Error
	if err != nil {
		DErrorE(err, "Database Credit Log Flush Error")
	}
}

func PushCreditLogs(groupId int64, cls ...CreditLog) {
	// get lock bank
	CreditLogBankLock.Lock()
	if CreditLogBanks == nil {
		CreditLogBanks = make(map[int64]*CreditLogBank)
	}

	var bank *CreditLogBank = nil
	var ok = false
	if bank, ok = CreditLogBanks[groupId]; bank == nil || !ok {
		bank = &CreditLogBank{
			Group: groupId,
			Logs:  make([]CreditLog, 0),
		}
		CreditLogBanks[groupId] = bank
	}
	CreditLogBankLock.Unlock()

	bank.updateLock.Lock()
	defer bank.updateLock.Unlock()

	bank.Logs = append(bank.Logs, cls...)
}

func FlushCreditLogs() {
	CreditLogBankLock.Lock()
	defer CreditLogBankLock.Unlock()

	for _, clb := range CreditLogBanks {
		go clb.Flush()
	}
}

func FindLog(groupId int64, logId int64) *CreditLog {
	ret := &CreditLog{}
	DB.Table(DBTName("Credit_Log", groupId)).First(&ret, logId)
	DInfof("Find Log | group=%d id=%d", groupId, logId)
	if ret != nil && ret.ID == logId {
		return ret
	}
	return nil
}

func QueryLogs(groupId int64, offset uint64, limit uint64, uid int64, before time.Time, vtype OPReasons) []CreditLog {
	var ret = []CreditLog{}

	tx := DB.Table(DBTName("Credit_Log", groupId)).Order("id DESC").Limit(int(limit)).Offset(int(offset))
	if uid > 0 {
		tx.Where("userid = ?", uid)
	}
	if vtype != "" {
		tx.Where("op = ?", string(vtype))
	}
	tx.Find(&ret)

	DInfof("Query Logs | group=%d offset=%d limit=%d userId=%d before=%d reason=%s columns=%d", groupId, offset, limit, uid, before.Unix(), vtype, len(ret))
	return ret
}

func init() {
	SetInterval(time.Second, func() {
		FlushCreditLogs()
	})
}
