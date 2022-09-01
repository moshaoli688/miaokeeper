package main

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/BBAlliance/miaokeeper/memutils"
	jsoniter "github.com/json-iterator/go"
	tb "gopkg.in/telebot.v3"
	"gorm.io/gorm/clause"
)

// GORM:%NAME%_Lottery
type DBLottery struct {
	ID        string    `gorm:"column:id;type:string;size:128;primaryKey;not null"`
	Config    string    `gorm:"column:config;type:text;not null"`
	CreatedAt time.Time `gorm:"column:createdat;autoCreateTime"`
}

// GORM:%NAME%_Lottery_Participation
type DBLotteryParticipation struct {
	LotteryID   string    `gorm:"column:lotteryid;type:string;size:128;uniqueIndex:uniq_participant;index:lottery_id;not null"`
	Participant int64     `gorm:"column:participant;uniqueIndex:uniq_participant;not null"`
	Username    string    `gorm:"column:username;type:text;not null"`
	CreatedAt   time.Time `gorm:"column:createdat;autoCreateTime"`
}

var LotteryConfigCache map[string]*LotteryInstance
var LotteryConfigLock sync.Mutex

// status: -1 not start, 0 start, 1 stopped, 2 finished, 3 cancelled
type LotteryInstance struct {
	ID        string
	Status    int
	GroupID   int64
	MsgID     int
	CreatedAt int64
	StartedAt int64

	Payload     string
	Limit       int
	Consume     bool
	Num         int
	Duration    time.Duration
	Participant int

	Winners          []DBLotteryParticipation
	ParticipantCache int        `json:"-"`
	JoinLock         sync.Mutex `json:"-"`
}

func (li *LotteryInstance) UpdateTelegramMsg() *tb.Message {
	btns := []string{}
	if li.Status == 0 {
		btns = append(btns, fmt.Sprintf("🤏 我要抽奖|lt?t=1&id=%s", li.ID))
	}
	if li.Status >= 0 && li.Status < 2 {
		btns = append(btns, fmt.Sprintf("📦 手动开奖[管理]|lt?t=3&id=%s||🗑️ 解散抽奖[管理]|lt?t=4&id=%s", li.ID, li.ID))
	}
	if li.Status == -1 {
		btns = append(btns, fmt.Sprintf("🎡 开启活动[管理]|lt?t=2&id=%s", li.ID))
	}
	if li.MsgID > 0 && li.Status == 2 {
		Bot.Delete(&tb.Message{ID: li.MsgID, Chat: &tb.Chat{ID: li.GroupID}})
		li.MsgID = 0
	}
	if li.MsgID <= 0 {
		msg, _ := SendBtnsMarkdown(&tb.Chat{ID: li.GroupID}, li.GenText(), "", btns)
		if msg != nil {
			li.MsgID = msg.ID
			li.Update()
		}
		return msg
	} else {
		msg, _ := EditBtnsMarkdown(&tb.Message{ID: li.MsgID, Chat: &tb.Chat{ID: li.GroupID}}, li.GenText(), "", btns)
		if msg == nil {
			li.MsgID = 0
			return li.UpdateTelegramMsg()
		}
		return msg
	}
}

func (li *LotteryInstance) GenText() string {
	drawMsg := ""
	if li.Participant > 0 {
		drawMsg = fmt.Sprintf("参与人数达 %d 人", li.Participant)
	}
	if li.Duration > 0 {
		if drawMsg != "" {
			drawMsg += " 或 "
		}
		durationStr := ""
		if li.Duration >= time.Hour {
			durationStr = fmt.Sprintf("%.1f 小时", li.Duration.Hours())
		} else {
			durationStr = fmt.Sprintf("%d 分钟", int(li.Duration.Minutes()))
		}
		drawMsg += fmt.Sprintf("%s后自动开奖", durationStr)
	}
	if drawMsg == "" {
		drawMsg = "手动开奖"
	}

	status := "`未知`"
	if li.Status == -1 {
		status = "`待验证`"
	} else if li.Status == 0 {
		status = "`进行中`"
	} else if li.Status == 1 {
		status = "`待手动开奖`"
	} else if li.Status == 2 {
		status = "`已开奖`"
	} else if li.Status == 3 {
		status = "`已废弃`"
	}
	if li.Status >= 0 {
		status += fmt.Sprintf("\n*参与人数:* %d", li.Participants())
	}
	if len(li.Winners) > 0 {
		status += "\n\n*🏆 获奖者:*"
		for i := range li.Winners {
			status += fmt.Sprintf("\n`%2d.` `%s` ([%d](%s))", i+1, GetQuotableStr(li.Winners[i].Username), li.Winners[i].Participant, fmt.Sprintf("tg://user?id=%d", li.Winners[i].Participant))
		}
	}

	return fmt.Sprintf(
		"🤖️ *抽奖任务:* `%s`.\n\n*抽奖配置:*\n积分要求: `%d`\n积分消耗: `%v`\n奖品数量: `%d`\n开奖方式: `%s`\n\n*任务状态:* %s",
		GetQuotableStr(li.Payload), li.Limit, li.Consume, li.Num, drawMsg, status,
	)
}

func (li *LotteryInstance) Update() bool {
	cfg, _ := jsoniter.Marshal(li)
	err := DB.Table(DBTName("Lottery")).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"config"}),
	}).Create(&DBLottery{
		ID:     li.ID,
		Config: string(cfg),
	}).Error

	if err != nil {
		DErrorf("Update Lottery Error | id=%s value=%s error=%v", li.ID, string(cfg), err.Error())
		return false
	}

	return true
}

func (li *LotteryInstance) Join(userId int64, username string) error {
	li.JoinLock.Lock()
	defer li.JoinLock.Unlock()

	if li.Status != 0 {
		return errors.New("❌ 抽奖活动不在有效时间范围内，请检查后再试 ~")
	}

	err := DB.Table(DBTName("Lottery_Participation")).Create(&DBLotteryParticipation{
		LotteryID:   li.ID,
		Participant: userId,
		Username:    username,
	}).Error

	if err != nil {
		DLogf("Join Lottery Error | id=%s user=%d error=%v", li.ID, userId, err.Error())
		return errors.New("❌ 您已经参加过这个活动了，请不要重复参加哦 ~")
	}

	if li.ParticipantCache > 0 {
		li.ParticipantCache += 1
	}

	return nil
}

func (li *LotteryInstance) Participants() int {
	if li.Status >= 0 {
		if li.ParticipantCache > 0 {
			return li.ParticipantCache
		}

		ret := int64(0)
		err := DB.Table(DBTName("Lottery_Participation")).Where("lotteryid = ?", li.ID).Count(&ret).Error
		if err != nil {
			DLogf("Fetch Lottery Participants Number Error | id=%s error=%v", li.ID, err.Error())
			return -1
		}

		li.ParticipantCache = int(ret)
		return li.ParticipantCache
	}
	return -1
}

func (li *LotteryInstance) StartLottery() {
	li.JoinLock.Lock()
	defer li.JoinLock.Unlock()

	if li.Status == -1 {
		li.Status = 0
		li.StartedAt = time.Now().Unix()
		li.Update()
		li.UpdateTelegramMsg()

		if li.Duration > 0 {
			lazyScheduler.After(li.Duration+time.Second, memutils.LSC("checkDraw", &CheckDrawArgs{
				LotteryId: li.ID,
			}))
		}
	}
}

func (li *LotteryInstance) Withdraw() {
	li.JoinLock.Lock()
	defer li.JoinLock.Unlock()

	if li.Status == 0 {
		// cancel draw
		li.Status = 3

		// cancel job
		// we do not record the async event id
		// and the event will check the status of the job.
		// so, currently we do not cancel job automatically

		// update database and telegram status
		li.Update()
		li.UpdateTelegramMsg()

		// return all credits to user
		// TODO: use batch update instead of ci updates
		// for better performance
		if li.Limit != 0 && li.Consume {
			participants := []DBLotteryParticipation{}
			DB.Table(DBTName("Lottery_Participation")).Where("lotteryid = ?", li.ID).Find(&participants)
			for _, participant := range participants {
				if ci := GetCreditInfo(li.GroupID, participant.Participant); ci != nil {
					ci.Update(UMAdd, int64(li.Limit), nil, OPByLotteryReverse, 0, fmt.Sprintf("ID=%v", li.ID))
				}
			}
		}
	}
}

func (li *LotteryInstance) CheckDraw(force bool) bool {
	li.JoinLock.Lock()
	defer li.JoinLock.Unlock()

	if li.Status == 0 {
		if force {
			// manual draw
			li.Status = 2
		} else if li.Duration > 0 && li.StartedAt > 0 && li.StartedAt+int64(li.Duration/time.Second) < time.Now().Unix() {
			// timeout draw
			li.Status = 2
		} else if li.Participant >= 0 && li.Participants() >= li.Participant {
			// participant exceeding draw
			li.Status = 2
		}

		// draw
		if li.Status == 2 {
			winners := []DBLotteryParticipation{}
			DB.Table(DBTName("Lottery_Participation")).Clauses(clause.OrderBy{
				Expression: clause.Expr{SQL: GetRandClause()},
			}).Where("lotteryid = ?", li.ID).Limit(li.Num).Find(&winners)

			li.Winners = winners
			li.Update()
			li.UpdateTelegramMsg()
			return true
		}
	}

	return false
}

func GetLottery(lotteryId string) *LotteryInstance {
	LotteryConfigLock.Lock()
	defer LotteryConfigLock.Unlock()
	if li, ok := LotteryConfigCache[lotteryId]; ok && li != nil {
		return li
	}

	ret := DBLottery{}
	err := DB.Table(DBTName("Lottery")).First(&ret, "id = ?", lotteryId).Error
	if err != nil {
		DErrorf("Fetch Lottery Error | id=%s error=%v", lotteryId, err.Error())
		return nil
	}

	li := LotteryInstance{}
	err = jsoniter.Unmarshal([]byte(ret.Config), &li)
	if err != nil {
		DErrorf("Unmarshal Lottery Error | id=%s error=%v", lotteryId, err.Error())
		return nil
	}

	LotteryConfigCache[li.ID] = &li
	return &li
}

func CreateLottery(groupId int64, payload string, limit int, consume bool, num int, duration int, participant int) *LotteryInstance {
	li := LotteryInstance{
		ID:        fmt.Sprintf("%d:%d:%d", Abs(groupId), time.Now().Unix(), rand.Intn(9999)),
		Status:    -1,
		GroupID:   groupId,
		CreatedAt: time.Now().Unix(),
		StartedAt: 0,

		Payload:     payload,
		Limit:       limit,
		Consume:     consume,
		Num:         num,
		Duration:    time.Minute * time.Duration(duration),
		Participant: participant,
	}

	if li.Update() {
		LotteryConfigLock.Lock()
		LotteryConfigCache[li.ID] = &li
		LotteryConfigLock.Unlock()
		return &li
	}
	return nil
}

func init() {
	LotteryConfigCache = make(map[string]*LotteryInstance)
}
