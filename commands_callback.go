package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	tb "gopkg.in/telebot.v3"
)

var callbackHandler *CallbackHandler

func CmdOnCallback(ctx tb.Context) error {
	return WarpError(func() {
		c := ctx.Callback()
		if c != nil {
			callbackHandler.Handle(c)
		}
	})
}

func InitCallback() {
	callbackHandler = &CallbackHandler{}

	callbackHandler.Add("close", func(cp *CallbackParams) {
		Bot.Delete(cp.Callback.Message)
	})

	callbackHandler.Add("msg", func(cp *CallbackParams) {
		msg, _ := cp.GetString("m")
		cp.Response(msg)
	}).Should("m", "string")

	callbackHandler.Add("user", func(cp *CallbackParams) {
		groupId, _ := cp.GetGroupId("c")
		userId, _ := cp.GetUserId("u")
		ci := GetCreditInfo(groupId, userId)
		if ci.ID == 0 {
			cp.Response("cmd.misc.user.notExist")
		} else {
			cp.Response(fmt.Sprintf("ID: %s\nName: %s\nCredit: %d", "@"+ci.Username, ci.Name, ci.Credit))
		}
	}).ShouldValidMiaoAdminOpt("c")

	callbackHandler.Add("vote", func(cp *CallbackParams) {
		gid, tuid := cp.GroupID(), cp.TriggerUserID()
		gc := cp.GroupConfig()
		uid, _ := cp.GetUserId("u")
		secuid, _ := cp.GetUserId("s")

		vtToken := fmt.Sprintf("vt-%d,%d", gid, uid)
		userVtToken := fmt.Sprintf("vu-%d,%d,%d", gid, uid, tuid)

		if _, ok := votemap.Get(vtToken); ok {
			if votemap.Add(userVtToken) == 1 {
				votes := votemap.Add(vtToken)
				if votes >= 6 {
					Unban(gid, uid, 0)
					votemap.Unset(vtToken)
					SmartEdit(cp.Callback.Message, cp.Callback.Message.Text+Locale("cb.unblock.byvote", cp.Locale()))
					addCredit(gid, &tb.User{ID: uid}, -gc.CreditMapping.Ban, true, OPByAbuse, secuid, "BanPunishmentRevoke")
					if secuid > 0 {
						addCredit(gid, &tb.User{ID: secuid}, -gc.CreditMapping.BanBouns, true, OPByAbuse, uid, "BanBonusRevoke")
					}
				} else {
					EditBtns(cp.Callback.Message, cp.Callback.Message.Text, "", GenVMBtns(votes, gid, uid, secuid))
				}
				cp.Response("cb.vote.success")
			} else {
				cp.Response("cb.vote.failure")
			}
		} else {
			cp.Response("cb.vote.notExists")
		}
	}).ShouldValidGroup(true).Should("u", "user")

	callbackHandler.Add("rp", func(cp *CallbackParams) {
		gid, tuid := cp.GroupID(), cp.TriggerUserID()
		rpKey, _ := cp.GetInt64("r")
		captcha, _ := cp.GetString("c")

		redpacketKey := fmt.Sprintf("%d-%d", gid, rpKey)
		credits, _ := redpacketmap.Get(redpacketKey)
		left, _ := redpacketnmap.Get(redpacketKey)
		if credits > 0 && left > 0 {
			redpacketBestKey := fmt.Sprintf("%d-%d:best", gid, rpKey)
			redpacketUserKey := fmt.Sprintf("%d-%d:%d", gid, rpKey, tuid)
			if redpacketmap.Add(redpacketUserKey) == 1 {
				if cap, ok := redpacketcaptcha.Get(redpacketKey); !ok || cap == "" || (captcha != "" && strings.HasPrefix(cap, captcha)) {
					amount := 0
					if left <= 1 {
						amount = credits
					} else if left == 2 {
						amount = rand.Intn(credits)
					} else {
						rate := 3
						if left <= 4 {
							rate = 2
						} else if left >= 12 {
							rate = 4
						}
						amount = rand.Intn(credits * rate / left)
					}
					redpacketnmap.Set(redpacketKey, left-1)
					redpacketmap.Set(redpacketKey, credits-amount)

					if amount == 0 {
						cp.Response("cb.rp.nothing")
					} else {
						lastBest, _ := redpacketmap.Get(redpacketBestKey)
						if amount > lastBest {
							redpacketmap.Set(redpacketBestKey, amount)
							redpacketrankmap.Set(redpacketBestKey, GetQuotableUserName(cp.TriggerUser()))
						}
						cp.Response(Locale("cb.rp.get.1", cp.TriggerUser().LanguageCode) + strconv.Itoa(amount) + Locale("cb.rp.get.2", cp.Locale()))
						addCredit(gid, cp.TriggerUser(), int64(amount), true, OPByRedPacket, rpKey/100000, "ReceiveRedPacket")
					}

					SendRedPacket(cp.Callback.Message, gid, rpKey, nil)
				} else {
					gc := cp.GroupConfig()
					if gc != nil && gc.RedPacketCaptchaFailCreditBehavior != 0 {
						addCredit(gid, cp.TriggerUser(), gc.RedPacketCaptchaFailCreditBehavior, true, OPByRedPacket, 0, "RedPacketCaptchaFailure")
					}
					cp.Response("cb.rp.captchaInvalid")
				}
			} else {
				cp.Response("cb.rp.duplicated")
			}
		} else {
			cp.Response("cb.rp.notExists")
		}
	}).ShouldValidGroup(true).Should("r", "int64").Lock("credit")

	callbackHandler.Add("unban", func(cp *CallbackParams) {
		gid := cp.GroupID()
		gc := cp.GroupConfig()
		uid, _ := cp.GetUserId("u")
		secuid, _ := cp.GetUserId("s")

		joinVerificationId := fmt.Sprintf("join,%d,%d", gid, uid)
		vtToken := fmt.Sprintf("vt-%d,%d", gid, uid)

		if Unban(gid, uid, 0) == nil {
			cp.Response("cb.unban.success")
		} else {
			cp.Response("cb.unban.failure")
		}
		SmartEdit(cp.Callback.Message, cp.Callback.Message.Text+Locale("cb.unblock.byadmin", cp.Locale()))
		joinmap.Unset(joinVerificationId)
		if secuid > 0 && votemap.Exist(vtToken) {
			addCredit(gid, &tb.User{ID: uid}, -gc.CreditMapping.Ban, true, OPByAbuse, secuid, "BanPunishmentRevoke")
			votemap.Unset(vtToken)
			addCredit(gid, &tb.User{ID: secuid}, -gc.CreditMapping.BanBouns, true, OPByAbuse, uid, "BanBonusRevoke")
		}
	}).ShouldValidGroupAdmin(true).Should("u", "user")

	callbackHandler.Add("kick", func(cp *CallbackParams) {
		gid := cp.GroupID()
		uid, _ := cp.GetUserId("u")

		joinVerificationId := fmt.Sprintf("join,%d,%d", gid, uid)
		vtToken := fmt.Sprintf("vt-%d,%d", gid, uid)

		if Kick(gid, uid) == nil {
			cp.Response("cb.kick.success")
		} else {
			cp.Response("cb.kick.failure")
		}
		joinmap.Unset(joinVerificationId)
		votemap.Unset(vtToken)
		SmartEdit(cp.Callback.Message, cp.Callback.Message.Text+Locale("cb.kicked.byadmin", cp.Locale()))
	}).ShouldValidGroupAdmin(true).Should("u", "user")

	callbackHandler.Add("check", func(cp *CallbackParams) {
		gid := cp.GroupID()
		uid, _ := cp.GetUserId("u")
		gc := cp.GroupConfig()

		joinVerificationId := fmt.Sprintf("join,%d,%d", gid, uid)

		if uid == cp.TriggerUserID() {
			usrStatus, _ := UserIsInGroup(gc.MustFollow, uid)
			if usrStatus == UIGIn {
				if Unban(gid, uid, 0) == nil {
					Bot.Delete(cp.Callback.Message)
					cp.Response("cb.validate.success")
					joinmap.Unset(joinVerificationId)
				} else {
					cp.Response("cb.validate.success.cannotUnban")
				}
			} else {
				cp.Response("cb.validate.failure")
			}
		} else {
			cp.Response("cb.validate.others")
		}
	}).ShouldValidGroup(true).Should("u", "user")

	callbackHandler.Add("lt", func(cp *CallbackParams) {
		// 1: lottery, 2: start, 3: draw
		cmdtype, _ := cp.GetInt64("t")
		lotteryId, _ := cp.GetString("id")
		li := GetLottery(lotteryId)
		if li != nil {
			isMiaoGroupAdmin := IsGroupAdminMiaoKo(cp.Callback.Message.Chat, cp.TriggerUser())
			if cmdtype == 2 && isMiaoGroupAdmin {
				li.StartLottery()
				cp.Response("cb.lottery.start")
			} else if cmdtype == 3 && isMiaoGroupAdmin {
				li.CheckDraw(true)
			} else if cmdtype == 1 {
				ci := GetCreditInfo(li.GroupID, cp.TriggerUserID())
				ci.Acquire(func() {
					if ci.Credit >= int64(li.Limit) {
						if err := li.Join(cp.TriggerUserID(), GetQuotableUserName(cp.TriggerUser())); err == nil {
							if li.Consume {
								ci.unsafeUpdate(UMAdd, -int64(li.Limit), (&UserInfo{}).From(li.GroupID, cp.TriggerUser()), OPByLottery, cp.TriggerUserID(), "LotteryConsume")
							}
							cp.Response("cb.lottery.enroll")
							if li.Participant > 0 {
								// check draw by particitant
								li.CheckDraw(false)
							}
							debouncer(func() {
								if li.Status == 0 {
									li.UpdateTelegramMsg()
								}
							})
						} else {
							cp.Response(err.Error())
						}
					} else {
						cp.Response("cb.lottery.noEnoughCredit")
					}
				})
			} else {
				cp.Response("cb.notMiaoAdmin")
			}
		} else {
			cp.Response("cb.noEvent")
		}
	}).ShouldValidGroup(true).Should("t", "int64").Should("id", "string")

	callbackHandler.Add("lg", func(cp *CallbackParams) {
		groupId, _ := cp.GetGroupId("c")
		userId, _ := cp.GetUserId("u")
		offset, _ := cp.GetInt64("o")
		limit, _ := cp.GetInt64("l")
		reason, _ := cp.GetString("t")
		mode, _ := cp.GetInt64("m")

		if offset < 0 || limit <= 0 || limit > 20 || mode < 0 {
			cp.Response("cmd.misc.outOfRange")
		} else {
			GenLogDialog(cp.Callback, nil, groupId, uint64(offset), uint64(limit), userId, cp.Callback.Message.Time(), OPParse(strings.ToUpper(reason)), uint64(mode))
		}
	}).ShouldValidMiaoAdminOpt("c").Should("o", "int64").Should("l", "int64")

}
