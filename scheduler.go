package main

import (
	"fmt"

	"github.com/BBAlliance/miaokeeper/memutils"
	tb "gopkg.in/tucnak/telebot.v2"
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
			Bot.Delete(&tb.Message{
				ID: args.MessageId,
				Chat: &tb.Chat{
					ID: args.ChatId,
				},
			})
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
			if joinmap.Exist(args.VerificationId) {
				cm, err := Bot.ChatMemberOf(fakeMsg.Chat, fakeMsg.Sender)
				if err != nil || cm.Role == tb.Restricted || cm.Role == tb.Kicked || cm.Role == tb.Left {
					KickOnce(fakeMsg.Chat.ID, fakeMsg.Sender.ID)
					SmartSend(fakeMsg.Chat, fmt.Sprintf(Locale("channel.kicked", GetSenderLocale(fakeMsg)), fakeMsg.Sender.ID), WithMarkdown())
				}
			}
		}
	})

	lazyScheduler.Reg("checkDraw", func(lsc *memutils.LazySchedulerCall) {
		args := CheckDrawArgs{}
		lsc.Arg(&args)
		if args.LotteryId != "" {
			li := GetLottery(args.LotteryId)
			if li.Status != 2 {
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
