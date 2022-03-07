package main

import (
	"fmt"
	"strings"

	tb "gopkg.in/telebot.v3"
)

func CmdOnText(m *tb.Message) {
	if gc := GetGroupConfig(m.Chat.ID); gc != nil {
		if !CheckChannelForward(m) {
			return
		}

		if !CheckChannelFollow(m, m.Sender, false) {
			return
		}

		if gc.ExecPolicy(m) {
			return
		}

		if CheckSpoiler(m) {
			RevealSpoiler(m)
			addCreditToMsgSender(m.Chat.ID, m, gc.CreditMapping.Command, true, OPNormal)
			return
		}

		if gc.IsBanKeyword(m) {
			CmdBanUser(m)
			return
		} else if gc.IsWarnKeyword(m) {
			CmdWarnUser(m)
			return
		}

		text := strings.TrimSpace(m.Text)
		textLen := len([]rune(text))
		userId := m.Sender.ID

		if puncReg.MatchString(text) {
			// commands
			addCreditToMsgSender(m.Chat.ID, m, gc.CreditMapping.Command, true, OPNormal)
		} else if lastID == userId && text == lastText {
			// duplicated messages
			addCreditToMsgSender(m.Chat.ID, m, gc.CreditMapping.Duplicated, true, OPNormal)
		} else if textLen >= 2 && (lastID != userId || (textLen >= 14 && text != lastText)) {
			// valid messages
			addCreditToMsgSender(m.Chat.ID, m, gc.CreditMapping.PerValidTextMessage, false, OPNormal)
			if ValidReplyUser(m) {
				addCreditToMsgSender(m.Chat.ID, m.ReplyTo, gc.CreditMapping.PerValidTextMessage, false, OPNormal)
			}
		}

		lastID = userId
		lastText = text
	}
}

func CmdOnSticker(m *tb.Message) {
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil {
		if !CheckChannelForward(m) {
			return
		}
		if !CheckChannelFollow(m, m.Sender, false) {
			return
		}

		userId := m.Sender.ID
		if lastID != userId {
			addCreditToMsgSender(m.Chat.ID, m, gc.CreditMapping.PerValidStickerMessage, false, OPNormal)
			if ValidReplyUser(m) {
				addCreditToMsgSender(m.Chat.ID, m.ReplyTo, gc.CreditMapping.PerValidStickerMessage, false, OPNormal)
			}
		}

		lastID = userId
	}
}

func CmdOnDocument(m *tb.Message) {
	if ok, _, session := ParseSession(m); ok && session != "" {
		switch session {
		case "Policy":
			CmdImportPolicy(m)
		}
		return
	}

	if m.Caption == "/su_import_credit" && m.Document != nil {
		CmdSuImportCredit(m)
	} else if m.Caption == "/import_policy" && m.Document != nil {
		CmdImportPolicy(m)
	} else {
		CheckChannelForward(m)
		CheckChannelFollow(m, m.Sender, false)
	}
}

func CmdOnUserLeft(m *tb.Message) {
	gc := GetGroupConfig(m.Chat.ID)
	if gc != nil && m.UserLeft.ID > 0 {
		gc.UpdateAdmin(m.UserLeft.ID, UMDel)
		UpdateCredit(BuildCreditInfo(m.Chat.ID, m.UserLeft, false), UMDel, 0, OPByCleanUp)
	}
	LazyDelete(m)
}

func CmdOnChatMember(ctx tb.Context) error {
	return WarpError(func() {
		cmu := ctx.ChatMember()
		gc := GetGroupConfig(cmu.Chat.ID)
		if gc != nil && cmu.NewChatMember != nil && cmu.NewChatMember.User != nil && cmu.NewChatMember.User.ID > 0 {
			user := cmu.NewChatMember.User
			if cmu.NewChatMember.Role == tb.Kicked ||
				cmu.NewChatMember.Role == tb.Left {
				gc.UpdateAdmin(user.ID, UMDel)
				UpdateCredit(BuildCreditInfo(cmu.Chat.ID, user, false), UMDel, 0, OPByCleanUp)
			}
		}
	})
}

func CmdOnChatJoinRequest(ctx tb.Context) error {
	return WarpError(func() {
		cjr := ctx.ChatJoinRequest()
		gc := GetGroupConfig(cjr.Chat.ID)
		if gc != nil && gc.UnderAttackMode {
			Bot.DeclineChatJoinRequest(cjr.Chat, cjr.Sender)
			textMsg := fmt.Sprintf(Locale("channel.kicked.underAttack", GetUserLocale(cjr.Chat, cjr.Sender)), cjr.Sender.ID)
			SmartSendDelete(cjr.Chat, textMsg, WithMarkdown())
		}
	})
}

func CmdOnUserJoined(m *tb.Message) {
	if gc := GetGroupConfig(m.Chat.ID); gc != nil {
		if gc.IsBlackListName(m.Sender) {
			KickOnce(m.Chat.ID, m.Sender.ID)
			SmartSend(m.Chat, fmt.Sprintf(Locale("channel.pattern.kicked", GetSenderLocale(m)), m.Sender.ID), WithMarkdown())
			return
		}
	}
	CheckChannelFollow(m, m.UserJoined, true)
}

func CmdOnPinned(m *tb.Message) {
	LazyDelete(m)
}

func CmdOnMisc(m *tb.Message) {
	CheckChannelForward(m)
	CheckChannelFollow(m, m.Sender, false)
}
