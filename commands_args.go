package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	tb "gopkg.in/telebot.v3"
)

func CmdSuExportCredit(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && IsAdmin(m.Sender.ID) {
		err := Bot.Notify(m.Sender, tb.UploadingDocument)
		if err != nil {
			DErrorE(err, "Cmd Error | Export Credit | Cannot notify user")
			SmartSendDelete(m, Locale("cmd.privateChatFirst", GetSenderLocale(m)))
			return
		}
		records := DumpCredits(m.Chat.ID)
		ioBuffer := bytes.Buffer{}
		w := csv.NewWriter(&ioBuffer)
		w.WriteAll(records)
		Bot.Send(m.Sender, &tb.Document{
			File:     tb.FromReader(&ioBuffer),
			MIME:     "text/csv",
			FileName: "CreditDump" + time.Now().Format(time.RFC3339) + ".csv",
		})
		SmartSendDelete(m, Locale("credit.exportSuccess", GetSenderLocale(m)))
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdImportPolicy(m *tb.Message) {
	defer Bot.Delete(m)
	chatId := m.Chat.ID
	if ok, cid, _ := ParseSession(m); ok {
		chatId = cid
	}
	gc := GetGroupConfig(chatId)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		Bot.Notify(m.Chat, tb.UploadingDocument)
		ioHandler, err := Bot.File(&m.Document.File)
		if err != nil {
			DErrorEf(err, "Import Credit Error | not downloaded url=%s", Bot.URL+"/file/bot"+Bot.Token+"/"+m.Document.FilePath)
			SmartSendDelete(m, Locale("policy.importError", GetSenderLocale(m)))
			return
		}
		data, _ := io.ReadAll(ioHandler)
		newGC := gc.Clone()
		err = jsoniter.Unmarshal(data, &newGC)
		if err != nil {
			DErrorE(err, "Import Credit Error | not parsed: "+err.Error())
			SmartSendDelete(m, Locale("policy.importParseError", GetSenderLocale(m)))
			return
		}
		newGC.Admins, newGC.ID, newGC.NameBlackListRegEx = gc.Admins, gc.ID, nil
		if SetGroupConfig(chatId, newGC.Check()) != nil {
			SmartSendDelete(m, Locale("policy.importSuccess", GetSenderLocale(m)))
		} else {
			SmartSendDelete(m, Locale("policy.importParseError", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSuImportCredit(m *tb.Message) {
	defer Bot.Delete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && IsAdmin(m.Sender.ID) {
		Bot.Notify(m.Chat, tb.UploadingDocument)
		ioHandler, err := Bot.File(&m.Document.File)
		if err != nil {
			DErrorEf(err, "Import Credit Error | not downloaded url=%s", Bot.URL+"/file/bot"+Bot.Token+"/"+m.Document.FilePath)
			SmartSendDelete(m, Locale("credit.importError", GetSenderLocale(m)))
			return
		}
		csvHandler := csv.NewReader(ioHandler)
		records, err := csvHandler.ReadAll()
		if err != nil {
			DErrorE(err, "Import Credit Error | not parsed")
			SmartSendDelete(m, Locale("credit.importParseError", GetSenderLocale(m)))
			return
		}
		FlushCredits(m.Chat.ID, records)
		SmartSendDelete(m, fmt.Sprintf(Locale("credit.importSuccess", GetSenderLocale(m)), len(records)))
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSuAddGroup(m *tb.Message) {
	defer LazyDelete(m)
	if IsAdmin(m.Sender.ID) && m.Chat.ID < 0 {
		if UpdateGroup(m.Chat.ID, UMAdd) {
			SmartSendDelete(m, Locale("su.group.addSuccess", GetSenderLocale(m)))
		} else {
			SmartSendDelete(m, Locale("su.group.addDuplicate", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSuDelGroup(m *tb.Message) {
	defer LazyDelete(m)
	if IsAdmin(m.Sender.ID) && m.Chat.ID < 0 {
		if UpdateGroup(m.Chat.ID, UMDel) {
			SmartSendDelete(m, Locale("su.group.delSuccess", GetSenderLocale(m)))
		} else {
			SmartSendDelete(m, Locale("su.group.delDuplicate", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSuQuitGroup(m *tb.Message) {
	defer LazyDelete(m)
	if IsAdmin(m.Sender.ID) && m.Chat.ID < 0 {
		UpdateGroup(m.Chat.ID, UMDel)
		Bot.Leave(m.Chat)
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSuAddAdmin(m *tb.Message) {
	defer LazyDelete(m)
	if IsAdmin(m.Sender.ID) {
		if ValidMessageUser(m.ReplyTo) {
			if UpdateAdmin(m.ReplyTo.Sender.ID, UMAdd) {
				SmartSendDelete(m.ReplyTo, Locale("grant.assign.success", GetSenderLocale(m)))
			} else {
				SmartSendDelete(m.ReplyTo, Locale("grant.assign.failure", GetSenderLocale(m)))
			}
		} else {
			SmartSendDelete(m, Locale("cmd.mustReply", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noPerm", GetSenderLocale(m)))
	}
}

func CmdSuDelAdmin(m *tb.Message) {
	defer LazyDelete(m)
	if IsAdmin(m.Sender.ID) {
		if ValidMessageUser(m.ReplyTo) {
			if UpdateAdmin(m.ReplyTo.Sender.ID, UMDel) {
				SmartSendDelete(m.ReplyTo, Locale("grant.dismiss.success", GetSenderLocale(m)))
			} else {
				SmartSendDelete(m.ReplyTo, Locale("grant.dismiss.failure", GetSenderLocale(m)))
			}
		} else {
			SmartSendDelete(m, Locale("cmd.mustReply", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noPerm", GetSenderLocale(m)))
	}
}

// ---------------- Group Admin ----------------

func CmdGetPolicy(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		err := Bot.Notify(m.Sender, tb.UploadingDocument)
		if err != nil {
			DErrorE(err, "Cmd Error | Export Policy | Cannot notify user")
			SmartSendDelete(m, Locale("cmd.privateChatFirst", GetSenderLocale(m)))
			return
		}
		Bot.Notify(m.Chat, tb.UploadingDocument)
		ioReader := strings.NewReader(gc.ToJson(true))
		Bot.Send(m.Sender, &tb.Document{
			File:     tb.FromReader(ioReader),
			MIME:     "application/json",
			FileName: fmt.Sprintf("Policy-%d-%s.json", Abs(m.Chat.ID), time.Now().Format(time.RFC3339)),
		})
		SmartSendDelete(m, Locale("policy.exportSuccess", GetSenderLocale(m)))
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSetPolicy(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		_, err := SmartSend(m.Sender, fmt.Sprintf(Locale("cmd.privateSession", GetSenderLocale(m)), GetQuotableChatName(m.Chat), m.Chat.ID, "Policy"), WithMarkdown())
		if err != nil {
			DErrorE(err, "Cmd Error | Import Policy | Cannot send user session")
			SmartSendDelete(m, Locale("cmd.privateChatFirst", GetSenderLocale(m)))
			return
		}
		SmartSendDelete(m, Locale("cmd.privateSession.sended", GetSenderLocale(m)))
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdGetToken(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		if _, err := SmartSend(m.Sender, fmt.Sprintf(Locale("cmd.getToken", GetSenderLocale(m)), GetQuotableChatName(m.Chat), m.Chat.ID, gc.GenerateSign(GST_API_SIGN), gc.GenerateSign(GST_POLICY_CALLBACK_SIGN)), WithMarkdown()); err == nil {
			SmartSendDelete(m, Locale("credit.exportSuccess", GetSenderLocale(m)))
		} else {
			SmartSendDelete(m, Locale("cmd.privateChatFirst", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdAddAdmin(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) && m.ReplyTo != nil {
		if gc.UpdateAdmin(m.ReplyTo.Sender.ID, UMAdd) {
			SmartSendDelete(m.ReplyTo, Locale("grant.assign.success", GetSenderLocale(m)))
		} else {
			SmartSendDelete(m.ReplyTo, Locale("grant.assign.failure", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSetConfig(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		extra := strings.Fields(m.Payload)
		if len(extra) != 2 {
			SmartSendDelete(m, fmt.Sprintf(Locale("system.wrongUsage", GetSenderLocale(m)), "/set <ConfigPath> <Value>"), WithMarkdown())
		} else if original, err := FieldWriter(gc, extra[0], extra[1], true); err != nil {
			SmartSendDelete(m, fmt.Sprintf(Locale("system.unexpectedError", GetSenderLocale(m)), err.Error()))
		} else {
			SmartSendDelete(m, fmt.Sprintf(Locale("cmd.misc.set.success", GetSenderLocale(m)), original, extra[1]), WithMarkdown())
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdGetConfig(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		extra := strings.Fields(m.Payload)
		if len(extra) != 1 {
			SmartSendDelete(m, fmt.Sprintf(Locale("system.wrongUsage", GetSenderLocale(m)), "/get <ConfigPath>"), WithMarkdown())
		} else if original, err := FieldWriter(gc, extra[0], "", false); err != nil {
			SmartSendDelete(m, fmt.Sprintf(Locale("system.unexpectedError", GetSenderLocale(m)), err.Error()))
		} else {
			SmartSendDelete(m, fmt.Sprintf(Locale("cmd.misc.get.success", GetSenderLocale(m)), extra[0], original), WithMarkdown())
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdDelAdmin(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) && m.ReplyTo != nil {
		if gc.UpdateAdmin(m.ReplyTo.Sender.ID, UMDel) {
			SmartSendDelete(m.ReplyTo, Locale("grant.dismiss.success", GetSenderLocale(m)))
		} else {
			SmartSendDelete(m.ReplyTo, Locale("grant.dismiss.failure", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdBanForward(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		isReply := false
		id, _ := strconv.ParseInt(m.Payload, 10, 64)
		if id == 0 && m.IsReply() && m.ReplyTo.IsForwarded() && m.ReplyTo.OriginalChat != nil {
			id = m.ReplyTo.OriginalChat.ID
			isReply = true
		}
		if id != 0 {
			if gc.UpdateBannedForward(id, UMAdd) {
				if isReply {
					Bot.Delete(m.ReplyTo)
				}
				SmartSendDelete(m, Locale("forward.ban.success", GetSenderLocale(m)))
			} else {
				SmartSendDelete(m, Locale("forward.ban.failure", GetSenderLocale(m)))
			}
		} else {
			SmartSendDelete(m, Locale("cmd.mustReplyChannelOrInput", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdUnbanForward(m *tb.Message) {
	defer LazyDelete(m)
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && (gc.IsAdmin(m.Sender.ID) || IsAdmin(m.Sender.ID)) {
		id, _ := strconv.ParseInt(m.Payload, 10, 64)
		if id == 0 && m.IsReply() && m.ReplyTo.IsForwarded() && m.ReplyTo.OriginalChat != nil {
			id = m.ReplyTo.OriginalChat.ID
		}
		if id != 0 {
			if gc.UpdateBannedForward(id, UMDel) {
				SmartSendDelete(m, Locale("forward.unban.success", GetSenderLocale(m)))
			} else {
				SmartSendDelete(m, Locale("forward.unban.failure", GetSenderLocale(m)))
			}
		} else {
			SmartSendDelete(m, Locale("cmd.mustReplyChannelOrInput", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSetCredit(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		addons := ParseStrToInt64Arr(strings.Join(strings.Fields(strings.TrimSpace(m.Payload)), ","))
		target := &CreditInfo{}
		credit := int64(0)

		if len(addons) == 0 {
			SmartSendDelete(m, Locale("credit.set.invalid", GetSenderLocale(m)))
			return
		}

		if len(addons) == 1 {
			credit = addons[0]
		} else {
			target.ID = addons[0]
			credit = addons[1]
		}

		if m.ReplyTo != nil {
			target = BuildCreditInfo(m.Chat.ID, m.ReplyTo.Sender, false)
		}
		target = UpdateCredit(target, UMSet, credit, OPByAdminSet)
		SmartSendDelete(m, fmt.Sprintf(Locale("credit.set.success", GetSenderLocale(m)), target.Credit))
	} else {
		SmartSendDelete(m, Locale("cmd.noMiaoPerm", GetSenderLocale(m)))
	}
}

func CmdAddCredit(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		addons := ParseStrToInt64Arr(strings.Join(strings.Fields(strings.TrimSpace(m.Payload)), ","))
		target := &CreditInfo{}
		credit := int64(0)

		if len(addons) == 0 {
			SmartSendDelete(m, Locale("credit.add.invalid", GetSenderLocale(m)))
			return
		}

		if len(addons) == 1 {
			credit = addons[0]
		} else {
			target.ID = addons[0]
			credit = addons[1]
		}

		if m.ReplyTo != nil {
			target = BuildCreditInfo(m.Chat.ID, m.ReplyTo.Sender, false)
		}
		target = UpdateCredit(target, UMAdd, credit, OPByAdmin)
		SmartSendDelete(m, fmt.Sprintf(Locale("credit.set.success", GetSenderLocale(m)), target.Credit))
	} else {
		SmartSendDelete(m, Locale("cmd.noMiaoPerm", GetSenderLocale(m)))
	}
}

func CmdCheckCredit(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		if m.Chat.ID > 0 || !m.IsReply() {
			SmartSendDelete(m, Locale("cmd.mustReply", GetSenderLocale(m)))
		} else {
			SmartSendDelete(m, fmt.Sprintf(Locale("credit.check.success", GetSenderLocale(m)), GetQuotableUserName(m.ReplyTo.Sender), GetCredit(m.Chat.ID, m.ReplyTo.Sender.ID).Credit), WithMarkdown())
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdSetAntiSpoiler(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		gc := GetGroupConfig(m.Chat.ID)
		if gc != nil {
			status := false

			if m.Payload == "on" {
				status = true
			} else if m.Payload == "off" {
				status = false
			} else {
				SmartSendDelete(m, Locale("spoiler.invalid", GetSenderLocale(m)))
				LazyDelete(m)
				return
			}

			gc.AntiSpoiler = status
			gc.Save()
			SmartSendDelete(m, fmt.Sprintf(Locale("spoiler.success", GetSenderLocale(m)), gc.AntiSpoiler), WithMarkdown())
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noMiaoPerm", GetSenderLocale(m)))
	}
}

func CmdSetChannel(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		gc := GetGroupConfig(m.Chat.ID)
		if gc != nil {
			payloads := strings.Fields(strings.TrimSpace(m.Payload))
			groupName := ""
			mode := ""
			if len(payloads) > 0 {
				groupName = payloads[0]
			}
			if len(payloads) > 1 {
				mode = payloads[1]
			}
			if groupName == "" {
				gc.MustFollow = ""
				gc.MustFollowOnJoin = false
				gc.MustFollowOnMsg = false
				gc.Save()
				SmartSendDelete(m, Locale("channel.set.cancel", GetSenderLocale(m)))
			} else {
				if inGroupStatus, _ := UserIsInGroup(groupName, Bot.Me.ID); inGroupStatus != UIGIn {
					SmartSendDelete(m, Locale("channel.cannotCheckChannel", GetSenderLocale(m)))
				} else {
					gc.MustFollow = groupName
					gc.MustFollowOnJoin = false
					gc.MustFollowOnMsg = false
					if mode == "join" {
						gc.MustFollowOnJoin = true
					} else if mode == "msg" {
						gc.MustFollowOnMsg = true
					} else {
						gc.MustFollowOnJoin = true
						gc.MustFollowOnMsg = true
					}
					gc.Save()
					SmartSendDelete(m, fmt.Sprintf(Locale("channel.set.success", GetSenderLocale(m)), gc.MustFollowOnJoin, gc.MustFollowOnMsg), WithMarkdown())
				}
			}
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noMiaoPerm", GetSenderLocale(m)))
	}
}

func CmdSetLocale(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		gc := GetGroupConfig(m.Chat.ID)
		if gc != nil {
			payloads := strings.Fields(strings.TrimSpace(m.Payload))
			if len(payloads) > 0 {
				if payloads[0] == "-" {
					gc.Locale = ""
				} else {
					gc.Locale = payloads[0]
				}
				gc.Save()
				SmartSendDelete(m, fmt.Sprintf(Locale("locale.set", GetSenderLocale(m)), gc.Locale))
			} else {
				SmartSendDelete(m, fmt.Sprintf(Locale("locale.get", GetSenderLocale(m)), gc.Locale))
			}
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noMiaoPerm", GetSenderLocale(m)))
	}
}

func CmdCreditRank(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		rank, _ := strconv.Atoi(m.Payload)
		if rank <= 0 {
			rank = 10
		} else if rank > 50 {
			rank = 50
		}
		ranks := GetCreditRank(m.Chat.ID, rank)
		rankStr := ""
		for i, c := range ranks {
			rankStr += fmt.Sprintf("`%2d`. `%s`: `%d`\n", i+1, strings.ReplaceAll(c.Name, "`", "'"), c.Credit)
		}
		SmartSend(m, Locale("credit.rank.info", GetSenderLocale(m))+rankStr, WithMarkdown())
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdCreditLog(m *tb.Message) {
	defer LazyDelete(m)
	_, ah := ArgParse(m.Payload)
	userId := int64(0)
	groupId := m.Chat.ID
	if gid, ok := ah.Int64("group"); ok && gid < 0 {
		groupId = gid
	}
	if m.IsReply() && m.ReplyTo.SenderChat == nil {
		userId = m.ReplyTo.Sender.ID
	}
	if IsGroupAdminMiaoKo(&tb.Chat{ID: groupId}, m.Sender) {
		if uid, ok := ah.Int64("user"); ok {
			userId = uid
		}
		reason, _ := ah.Str("reason")

		GenLogDialog(nil, m, groupId, 0, 10, userId, time.Now(), OPParse(strings.ToUpper(reason)), 0)
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdCreateLottery(m *tb.Message) {
	// :limit=(0-inf)
	// :consume=n|y
	// :num=1|100
	// :duration=(0-3*24*60min)
	// :participant=(0-inf)
	// :pin=y|n
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		payload, ah := ArgParse(m.Payload)
		limit, _ := ah.Int("limit")
		consume, _ := ah.Bool("consume")
		num, _ := ah.Int("num")
		if num <= 0 || num >= 100 {
			num = 1
		}
		duration, _ := ah.Int("duration")
		if duration <= 0 || duration >= 3*24*60 {
			duration = 0
		}
		participant, _ := ah.Int("participant")
		if participant < num {
			participant = 0
		}
		pin, setPin := ah.Bool("pin")
		if !setPin {
			pin = true
		}

		li := CreateLottery(m.Chat.ID, payload, limit, consume, num, duration, participant)

		if li != nil {
			msg := li.UpdateTelegramMsg()
			if msg != nil && pin {
				Bot.Pin(msg)
			}
		} else {
			SmartSendDelete(m, Locale("system.unexpected", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdLottery(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdminMiaoKo(m.Chat, m.Sender) {
		payloads := strings.Fields(m.Payload)

		rank := 0
		if len(payloads) > 0 {
			rank, _ = strconv.Atoi(payloads[0])
		}
		n := 0
		if len(payloads) > 1 {
			n, _ = strconv.Atoi(payloads[1])
		}

		if rank <= 0 {
			rank = 10
		} else if rank > 100 {
			rank = 100
		}
		if n > rank {
			n = rank
		}

		ranks := GetCreditRank(m.Chat.ID, rank)
		sort.Slice(ranks, func(i, j int) bool {
			return rand.Intn(10) >= 5
		})
		rankStr := ""
		for i, c := range ranks[:n] {
			rankStr += fmt.Sprintf("`%2d.` `%s` ([%d](%s))\n", i+1, strings.ReplaceAll(c.Name, "`", "'"), c.ID, fmt.Sprintf("tg://user?id=%d", c.ID))
		}
		SmartSend(m, fmt.Sprintf(Locale("credit.lottery.info", GetSenderLocale(m))+rankStr), WithMarkdown())
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdBanUserCommand(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdmin(m.Chat, m.Sender) && ValidReplyUser(m) {
		if err := Ban(m.Chat.ID, m.ReplyTo.Sender.ID, 0); err == nil {
			SmartSendDelete(m, fmt.Sprintf(Locale("gp.ban.success", GetSenderLocale(m)), GetQuotableUserName(m.ReplyTo.Sender)), WithMarkdown())
		} else {
			DErrorE(err, "Perm Update | Fail to ban user")
			SmartSendDelete(m, Locale("gp.ban.failure", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noPerm", GetSenderLocale(m)))
	}
}

func CmdUnbanUserCommand(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdmin(m.Chat, m.Sender) && ValidReplyUser(m) {
		if err := Unban(m.Chat.ID, m.ReplyTo.Sender.ID, 0); err == nil {
			SmartSendDelete(m, fmt.Sprintf(Locale("gp.unban.success", GetSenderLocale(m)), GetQuotableUserName(m.ReplyTo.Sender)), WithMarkdown())
		} else {
			DErrorE(err, "Perm Update | Fail to unban user")
			SmartSendDelete(m, Locale("gp.unban.failure", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noPerm", GetSenderLocale(m)))
	}
}

func CmdKickUserCommand(m *tb.Message) {
	defer LazyDelete(m)
	if IsGroupAdmin(m.Chat, m.Sender) && ValidReplyUser(m) {
		if err := KickOnce(m.Chat.ID, m.ReplyTo.Sender.ID); err == nil {
			SmartSendDelete(m, fmt.Sprintf(Locale("gp.kick.success", GetSenderLocale(m)), GetQuotableUserName(m.ReplyTo.Sender)), WithMarkdown())
		} else {
			DErrorE(err, "Perm Update | Fail to kick user once")
			SmartSendDelete(m, Locale("gp.kick.failure", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noPerm", GetSenderLocale(m)))
	}
}

// ---------------- Normal User ----------------

func CmdRedpacket(m *tb.Message) {
	defer LazyDelete(m)
	if gc := GetGroupConfig(m.Chat.ID); gc != nil {
		if gc.ExecPolicy(m) {
			return
		}

		payloads := strings.Fields(m.Payload)

		mc := 0
		if len(payloads) > 0 {
			mc, _ = strconv.Atoi(payloads[0])
		}
		n := 0
		if len(payloads) > 1 {
			n, _ = strconv.Atoi(payloads[1])
		}

		if mc <= 0 || n <= 0 || mc > 100000 || n > 100 || mc < n {
			SmartSendDelete(m, Locale("rp.set.invalid", GetSenderLocale(m)), WithMarkdown())
			LazyDelete(m)
			return
		}

		usercreditlock.Lock()
		defer usercreditlock.Unlock()
		ci := GetCredit(m.Chat.ID, m.Sender.ID)

		if ci != nil && ci.Credit >= int64(mc) {
			chatId := m.Chat.ID
			addCredit(chatId, m.Sender, -Abs(int64(mc)), true, OPByRedPacket)
			redpacketId := time.Now().Unix() + int64(rand.Intn(10000))
			redpacketKey := fmt.Sprintf("%d-%d", chatId, redpacketId)
			redpacketrankmap.Set(redpacketKey+":sender", GetQuotableUserName(m.Sender))
			redpacketmap.Set(redpacketKey, mc)
			redpacketnmap.Set(redpacketKey, n)
			var buf *bytes.Buffer
			if gc.RedPacketCaptcha {
				buffer, results := GenerateRandomCaptcha()
				redpacketcaptcha.Set(redpacketKey, strings.Join(results, ","))
				buf = buffer
			}
			SendRedPacket(m.Chat, chatId, redpacketId, buf)
		} else {
			SmartSendDelete(m, Locale("rp.set.noEnoughCredit", GetSenderLocale(m)))
		}
	} else {
		SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
	}
}

func CmdMyCredit(m *tb.Message) {
	defer LazyDelete(m)
	if m.Chat.ID > 0 {
		SmartSendDelete(m, Locale("cmd.mustInGroup", GetSenderLocale(m)))
	} else if gc := GetGroupConfig(m.Chat.ID); gc != nil {
		if gc.ExecPolicy(m) {
			return
		}

		SmartSendDelete(m, fmt.Sprintf(Locale("credit.check.my", GetSenderLocale(m)), GetQuotableUserName(m.Sender), GetCredit(m.Chat.ID, m.Sender.ID).Credit), WithMarkdown())
	}
}

func CmdCreditTransfer(m *tb.Message) {
	defer LazyDelete(m)
	if m.Chat.ID > 0 {
		SmartSendDelete(m, Locale("cmd.mustInGroup", GetSenderLocale(m)))
	} else if gc := GetGroupConfig(m.Chat.ID); gc != nil {
		if gc.ExecPolicy(m) {
			return
		}

		credit, _ := strconv.Atoi(m.Payload)
		if credit <= 0 || !ValidReplyUser(m) {
			SmartSendDelete(m, Locale("transfer.invalidParam", GetSenderLocale(m)))
		} else {
			usercreditlock.Lock()
			defer usercreditlock.Unlock()

			ci := GetCredit(m.Chat.ID, m.Sender.ID)
			if ci.Credit >= int64(credit) {
				addCredit(m.Chat.ID, m.Sender, -int64(credit), true, OPByTransfer)
				addCredit(m.Chat.ID, m.ReplyTo.Sender, int64(credit), true, OPByTransfer)

				SmartSendDelete(m, fmt.Sprintf(Locale("transfer.success", GetSenderLocale(m)), credit))
			} else {
				SmartSendDelete(m, Locale("transfer.noBalance", GetSenderLocale(m)))
			}
		}
	}
}

func CmdVersion(m *tb.Message) {
	defer LazyDelete(m)
	SmartSendDelete(m, fmt.Sprintf(Locale("cmd.misc.version", GetSenderLocale(m)), version))
}

func CmdInfo(m *tb.Message) {
	defer LazyDelete(m)
	retStr := ""
	usrStatus, usrGroupStatus := UIGStatus("N/A"), tb.MemberStatus("N/A")
	if m.ReplyTo != nil {
		if m.ReplyTo.SenderChat != nil {
			retStr = fmt.Sprintf(Locale("cmd.misc.replyid.chat", GetSenderLocale(m)), m.Chat.ID, m.ReplyTo.SenderChat.ID, m.ReplyTo.SenderChat.Type)
		} else {
			if gc := GetGroupConfig(m.Chat.ID); gc != nil {
				usrStatus, usrGroupStatus = UserIsInGroup(gc.MustFollow, m.ReplyTo.Sender.ID)
			}
			retStr = fmt.Sprintf(Locale("cmd.misc.replyid.user", GetSenderLocale(m)), m.Chat.ID, m.ReplyTo.Sender.ID, m.ReplyTo.Sender.LanguageCode, usrStatus, usrGroupStatus)
		}
	} else {
		if m.SenderChat != nil {
			retStr = fmt.Sprintf(Locale("cmd.misc.id.chat", GetSenderLocale(m)), m.Chat.ID, m.SenderChat.ID, m.SenderChat.Type)
		} else {
			if gc := GetGroupConfig(m.Chat.ID); gc != nil {
				usrStatus, usrGroupStatus = UserIsInGroup(gc.MustFollow, m.Sender.ID)
			}
			retStr = fmt.Sprintf(Locale("cmd.misc.id.user", GetSenderLocale(m)), m.Chat.ID, m.Sender.ID, m.Sender.LanguageCode, usrStatus, usrGroupStatus)
		}
	}
	SmartSendDelete(m, retStr, WithMarkdown())
}

func CmdPing(m *tb.Message) {
	defer LazyDelete(m)
	t := time.Now().UnixMilli()
	Bot.Commands()
	t1 := time.Now().UnixMilli() - t
	msg, _ := SmartSendDelete(m.Chat, fmt.Sprintf(Locale("cmd.misc.ping.1", GetSenderLocale(m)), t1), WithMarkdown())
	t2 := time.Now().UnixMilli() - t - t1
	SmartEdit(msg, fmt.Sprintf(Locale("cmd.misc.ping.2", GetSenderLocale(m)), t1, t2), WithMarkdown())
}
