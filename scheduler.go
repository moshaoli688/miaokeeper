package main

import (
	"fmt"

	"github.com/BBAlliance/miaokeeper/memutils"
	tb "gopkg.in/telebot.v3"
)

type DeleteMessageArgs struct {
	ChatId    int64
	MessageId int
}

type InGroupVerifyArgs struct {
	ChatId         int64
	UserId         int64
	MessageId      int
	VerificationId string
	LanguageCode   string
}

type CheckDrawArgs struct {
	LotteryId string
}

type UnbanUserArgs struct {
	ChatId int64
	UserId int64
}

func InitScheduler() {
	lazyScheduler.Reg("deleteMessage", func(lsc *memutils.LazySchedulerCall) {
		args := DeleteMessageArgs{}
		lsc.Arg(&args)
		if args.ChatId != 0 && args.MessageId != 0 {
			if err := Bot.Delete(&tb.Message{
				ID: args.MessageId,
				Chat: &tb.Chat{
					ID: args.ChatId,
				},
			}); err != nil {
				DWarnf("Scheduler Warning | method=deleteMessage warning=deleteMessageFailed details=%s", err.Error())
			}
		}
	})

	lazyScheduler.Reg("inGroupVerify", func(lsc *memutils.LazySchedulerCall) {
		args := InGroupVerifyArgs{}
		lsc.Arg(&args)
		if args.VerificationId != "" {
			fakeMsg := &tb.Message{
				ID: args.MessageId,
				Chat: &tb.Chat{
					ID: args.ChatId,
				},
				Sender: &tb.User{
					ID:           args.UserId,
					LanguageCode: args.LanguageCode,
				},
			}
			Bot.Delete(fakeMsg)
			underAttackMode := false
			if gc := GetGroupConfig(args.ChatId); gc != nil {
				underAttackMode = gc.UnderAttackMode
			}
			if joinmap.Exist(args.VerificationId) {
				cm, err := Bot.ChatMemberOf(fakeMsg.Chat, fakeMsg.Sender)
				if err != nil || cm.Role == tb.Restricted || cm.Role == tb.Kicked || cm.Role == tb.Left {
					textMsg := fmt.Sprintf(Locale("channel.kicked", GetSenderLocale(fakeMsg)), fakeMsg.Sender.ID)
					if underAttackMode {
						Kick(fakeMsg.Chat.ID, fakeMsg.Sender.ID)
						SmartSendDelete(fakeMsg.Chat, textMsg, WithMarkdown())
					} else {
						KickOnce(fakeMsg.Chat.ID, fakeMsg.Sender.ID)
						SmartSendDeleteAfter(fakeMsg.Chat, textMsg, 5*60, WithMarkdown())
					}
				}
			}
		}
	})

	lazyScheduler.Reg("checkDraw", func(lsc *memutils.LazySchedulerCall) {
		args := CheckDrawArgs{}
		lsc.Arg(&args)
		if args.LotteryId != "" {
			li := GetLottery(args.LotteryId)
			if li.Status == 0 {
				li.CheckDraw(false)
			}
		}
	})

	lazyScheduler.Reg("unbanUser", func(lsc *memutils.LazySchedulerCall) {
		args := UnbanUserArgs{}
		lsc.Arg(&args)
		if args.ChatId != 0 && args.UserId != 0 {
			Bot.Unban(&tb.Chat{ID: args.ChatId}, &tb.User{ID: args.UserId}, true)
		}
	})
}
