package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BBAlliance/miaokeeper/memutils"
	jsoniter "github.com/json-iterator/go"
	tb "gopkg.in/tucnak/telebot.v2"
)

func SendRedPacket(to interface{}, chatId int64, packetId int64, photo *bytes.Buffer) (*tb.Message, error) {
	redpacketKey := fmt.Sprintf("%d-%d", chatId, packetId)
	credits, _ := redpacketmap.Get(redpacketKey)
	left, _ := redpacketnmap.Get(redpacketKey)
	sender, _ := redpacketrankmap.Get(redpacketKey + ":sender")
	captcha, _ := redpacketcaptcha.Get(redpacketKey)

	msg := fmt.Sprintf(Locale("rp.complete", "zh"), sender)
	btns := []string{}
	hasLeft := credits > 0 && left > 0

	if hasLeft {
		creditLeft := strconv.Itoa(credits)
		if left == 1 {
			creditLeft = Locale("rp.guessLeft", "zh")
		}
		msg = fmt.Sprintf(Locale("rp.text", "zh"), sender, creditLeft, left)
		if captcha == "" {
			btns = []string{fmt.Sprintf(Locale("btn.rp.draw", "zh"), packetId)}
		} else {
			msg += Locale("rp.text.captcha", "zh")
			results := strings.Split(captcha, ",")
			sort.Strings(results)
			for _, s := range results {
				btns = append(btns, fmt.Sprintf(Locale("btn.rp.draw.captcha", "zh"), s, s, packetId))
			}
			btns = []string{strings.Join(btns, "||")}
		}
	}

	redpacketBestKey := fmt.Sprintf("%d-%d:best", chatId, packetId)
	if lastBest, _ := redpacketmap.Get(redpacketBestKey); lastBest > 0 {
		bestDrawer, _ := redpacketrankmap.Get(redpacketBestKey)
		msg += fmt.Sprintf(Locale("rp.lucky", "zh"), bestDrawer, lastBest)
	}

	if Type(to) == "*telebot.Message" {
		mess, _ := to.(*tb.Message)
		if mess.Photo != nil {
			if !hasLeft {
				Bot.Delete(mess)
				return SendBtnsMarkdown(mess.Chat, msg, "", btns)
			}
			return EditBtnsMarkdown(mess, &tb.Photo{
				File:    mess.Photo.File,
				Caption: msg,
			}, "", btns)
		} else {
			return EditBtnsMarkdown(mess, msg, "", btns)
		}
	} else {
		if photo != nil {
			return SendBtnsMarkdown(to, &tb.Photo{
				File:    tb.FromReader(photo),
				Caption: msg,
			}, "", btns)
		}
		return SendBtnsMarkdown(to, msg, "", btns)
	}
}

func CheckChannelForward(m *tb.Message) bool {
	if m.IsForwarded() {
		if gc := GetGroupConfig(m.Chat.ID); gc != nil && len(gc.BannedForward) > 0 {
			shouldDelete := (m.OriginalChat != nil && gc.IsBannedForward(m.OriginalChat.ID)) ||
				(m.OriginalSender != nil && gc.IsBannedForward(m.OriginalSender.ID))
			if shouldDelete {
				Bot.Delete(m)
				return false
			}
		}
	}
	return true
}

func CheckSpoiler(m *tb.Message) bool {
	if gc := GetGroupConfig(m.Chat.ID); gc != nil && gc.AntiSpoiler {
		for _, e := range m.Entities {
			if e.Type == "spoiler" {
				return true
			}
		}
	}
	return false
}

func CheckChannelFollow(m *tb.Message, user *tb.User, isJoin bool) bool {
	showExceptDialog := isJoin
	if gc := GetGroupConfig(m.Chat.ID); gc != nil && gc.MustFollow != "" {
		if isJoin && !gc.MustFollowOnJoin {
			return true
		}
		if !isJoin && !gc.MustFollowOnMsg {
			return true
		}
		usrName := GetQuotableUserName(user)

		// ignore bot
		if user.IsBot {
			if showExceptDialog {
				SmartSendDelete(m.Chat, fmt.Sprintf(Locale("channel.bot.permit", GetSenderLocale(m)), usrName))
			}
			return true
		}

		// ignore channel
		if !ValidMessageUser(m) {
			return true
		}

		usrStatus := UserIsInGroup(gc.MustFollow, user.ID)
		if usrStatus == UIGIn {
			if showExceptDialog {
				SmartSendDelete(m.Chat, fmt.Sprintf(Locale("channel.user.alreadyFollowed", GetSenderLocale(m)), usrName))
			}
		} else if usrStatus == UIGOut {
			chatId, userId := m.Chat.ID, user.ID
			joinVerificationId := fmt.Sprintf("join,%d,%d", chatId, userId)
			if joinmap.Add(joinVerificationId) > 1 {
				// already in verification process
				Bot.Delete(m)
				return false
			}
			msg, err := SendBtnsMarkdown(m.Chat, fmt.Sprintf(Locale("channel.request", GetSenderLocale(m)), userId, usrName), "", []string{
				fmt.Sprintf(Locale("btn.channel.step1", GetSenderLocale(m)), strings.TrimLeft(gc.MustFollow, "@")),
				fmt.Sprintf(Locale("btn.channel.step2", GetSenderLocale(m)), userId),
				fmt.Sprintf(Locale("btn.adminPanel", GetSenderLocale(m)), userId, 0, userId, 0),
			})
			if msg == nil || err != nil {
				if showExceptDialog {
					SmartSendDelete(m.Chat, Locale("channel.cannotSendMsg", GetSenderLocale(m)))
				}
				joinmap.Unset(joinVerificationId)
			} else {
				if Ban(chatId, userId, 0) != nil {
					LazyDelete(msg)
					if showExceptDialog {
						SmartSendDelete(m.Chat, Locale("channel.cannotBanUser", GetSenderLocale(m)))
					}
					joinmap.Unset(joinVerificationId)
				} else {
					lazyScheduler.After(time.Minute*5, memutils.LSC("inGroupVerify", &InGroupVerifyArgs{
						ChatId:         chatId,
						UserId:         userId,
						MessageId:      msg.ID,
						VerificationId: joinVerificationId,
						LanguageCode:   user.LanguageCode,
					}))
					Bot.Delete(m)
					return false
				}
			}
		} else {
			if showExceptDialog {
				SmartSendDelete(m.Chat, Locale("channel.cannotCheckChannel", GetSenderLocale(m)))
			}
		}
	}
	return true
}

func Rsp(c *tb.Callback, msg string) {
	Bot.Respond(c, &tb.CallbackResponse{
		Text:      Locale(msg, c.Sender.LanguageCode),
		ShowAlert: true,
	})
}

func GenVMBtns(votes int, chatId, userId, secondUserId int64) []string {
	return []string{
		fmt.Sprintf(Locale("btn.notFair", "zh"), votes, userId, secondUserId),
		fmt.Sprintf(Locale("btn.adminPanel", "zh"), userId, secondUserId, userId, secondUserId),
	}
}

func GenLogDialog(c *tb.Callback, m *tb.Message, groupId int64, offset uint64, limit uint64, userId int64, before time.Time, reason OPReasons, dialogMode uint64) {
	if c == nil && m == nil {
		return
	}
	var operator *tb.User = nil
	inGroup := false
	locale := ""
	if c == nil {
		operator = m.Sender
		locale = GetSenderLocale(m)
		_, ah := ArgParse(m.Payload)
		inGroup, _ = ah.Bool("ingroup")
		// check private message sending permission
		if m.Chat.ID < 0 && !inGroup {
			err := Bot.Notify(m.Sender, tb.UploadingDocument)
			if err != nil {
				SmartSendDelete(m, Locale("cmd.privateChatFirst", GetSenderLocale(m)))
				return
			}
		}
	} else {
		operator = c.Sender
		locale = GetSenderLocaleCallback(c)
	}

	// permission check
	if !IsGroupAdminMiaoKo(&tb.Chat{ID: groupId}, operator) {
		if c != nil {
			Rsp(c, "cb.notMiaoAdmin")
		} else {
			SmartSendDelete(m, Locale("cmd.noGroupPerm", GetSenderLocale(m)))
		}
		return
	}

	// dialog mode
	isToggle := dialogMode == 1

	toggleButtonStr := ""
	toggleButtonMode := 0

	// build message
	text := ""
	var logs []CreditLog = nil

	if !isToggle {
		nlen := 0
		logs = QueryLogs(groupId, offset, limit, userId, before, reason)
		for _, r := range logs {
			nlen = MaxInt(nlen, len(fmt.Sprintf("%d", r.Credit)))
		}
		for _, r := range logs {
			text += fmt.Sprintf("`%10d` | `%s(%"+fmt.Sprintf("%d", nlen)+"d)` | `%s`\n", r.UserID, r.Reason[:1], r.Credit, r.CreatedAt.Format("01-02.15:04"))
		}
		toggleButtonMode = 1
	} else {
		text = fmt.Sprintf("`/creditlog@%s :ingroup=true :group=%d :user=%d :reason=%s`", Bot.Me.Username, groupId, userId, reason)
		toggleButtonStr = "· "
	}

	userRepr := "N/A"
	if userId > 0 {
		userRepr = fmt.Sprintf("%d", userId)
	}

	buttons := []string{
		fmt.Sprintf("👤 %s|user?c=%d&u=%d||🔍 %s|msg?m=%s", userRepr, groupId, userId, reason.Repr(), reason.Repr()),
		Locale("cmd.misc.prevPage", locale) + fmt.Sprintf("|lg?c=%d&o=%d&l=%d&u=%d&t=%s||", groupId, int64(offset)-int64(limit), limit, userId, reason) +
			toggleButtonStr + fmt.Sprintf(Locale("cmd.misc.atPage", locale), offset/limit+1) + fmt.Sprintf("|lg?c=%d&o=%d&l=%d&u=%d&t=%s&m=%d||", groupId, offset, limit, userId, reason, toggleButtonMode) +
			Locale("cmd.misc.nextPage", locale) + fmt.Sprintf("|lg?c=%d&o=%d&l=%d&u=%d&t=%s", groupId, offset+limit, limit, userId, reason),
	}

	if c == nil {
		if inGroup {
			// send directly in group
			SmartSendWithBtns(m.Chat, fmt.Sprintf(Locale("cmd.credit.logHead", locale), groupId, text), buttons, WithMarkdown())
		} else {
			// send to private chat
			SmartSendWithBtns(operator, fmt.Sprintf(Locale("cmd.credit.logHead", locale), groupId, text), buttons, WithMarkdown())
		}
	} else {
		if len(logs) == 0 && !isToggle {
			Rsp(c, "cmd.misc.outOfRange")
		} else {
			_, err := EditBtnsMarkdown(c.Message, fmt.Sprintf(Locale("cmd.credit.logHead", locale), groupId, text), "", buttons)
			if err != nil {
				Rsp(c, "cmd.misc.noChange")
			}
		}
	}
}

func addCreditToMsgSender(chatId int64, m *tb.Message, credit int64, force bool, reason OPReasons) *CreditInfo {
	if ValidMessageUser(m) {
		return addCredit(chatId, m.Sender, credit, force, reason)
	}
	return nil
}

func addCredit(chatId int64, user *tb.User, credit int64, force bool, reason OPReasons) *CreditInfo {
	if chatId < 0 && user != nil && user.ID > 0 && credit != 0 {
		token := fmt.Sprintf("ac-%d-%d", chatId, user.ID)
		if creditomap.Add(token) < 20 || force { // can only get credit 20 times / hour
			return UpdateCredit(BuildCreditInfo(chatId, user, false), UMAdd, credit, reason)
		}
	}
	return nil
}

func ValidReplyUser(m *tb.Message) bool {
	return m.ReplyTo != nil && ValidMessageUser(m) && ValidMessageUser(m.ReplyTo) && m.ReplyTo.Sender.ID != m.Sender.ID
}

func ValidMessageUser(m *tb.Message) bool {
	return m != nil && m.SenderChat == nil && ValidUser(m.Sender)
}

func ValidUser(u *tb.User) bool {
	return u != nil && u.ID > 0 && !u.IsBot && u.ID != 777000 && u.Username != "Channel_Bot" && u.Username != "GroupAnonymousBot" && u.Username != "Telegram"
}

func BuildCreditInfo(groupId int64, user *tb.User, autoFetch bool) *CreditInfo {
	ci := &CreditInfo{
		user.Username, GetUserName(user), user.ID, 0, groupId,
	}
	if autoFetch {
		ci.Credit = GetCredit(groupId, user.ID).Credit
	}
	return ci
}

func SmartEdit(to *tb.Message, what interface{}, options ...interface{}) (*tb.Message, error) {
	if len(options) == 0 {
		options = append([]interface{}{&tb.SendOptions{
			DisableWebPagePreview: true,
			AllowWithoutReply:     true,
		}}, options...)
	}
	m, err := Bot.Edit(to, what, options...)
	if err != nil {
		DErrorE(err, "Telegram Edit Error")
	}
	return m, err
}

func SmartSendDelete(to interface{}, what interface{}, options ...interface{}) (*tb.Message, error) {
	msg, err := SmartSend(to, what, options...)
	if err == nil && msg != nil {
		LazyDelete(msg)
	}
	return msg, err
}

func MakeBtns(prefix string, btns []string) [][]tb.InlineButton {
	btnsc := make([][]tb.InlineButton, 0)
	for _, row := range btns {
		btnscr := make([]tb.InlineButton, 0)
		for _, btn := range strings.Split(row, "||") {
			z := strings.SplitN(btn, "|", 2)
			if len(z) < 2 {
				continue
			}
			unique := ""
			link := ""
			if _, err := url.Parse(z[1]); err == nil && strings.HasPrefix(z[1], "https://") {
				link = z[1]
			} else {
				unique = prefix + z[1]
			}
			btnscr = append(btnscr, tb.InlineButton{
				Unique: unique,
				Text:   z[0],
				Data:   "",
				URL:    link,
			})
		}
		btnsc = append(btnsc, btnscr)
	}
	return btnsc
}

func SendBtns(to interface{}, what interface{}, prefix string, btns []string) (*tb.Message, error) {
	return SmartSendInner(to, what, &tb.SendOptions{
		DisableWebPagePreview: true,
		AllowWithoutReply:     true,
	}, &tb.ReplyMarkup{
		OneTimeKeyboard:     true,
		ResizeReplyKeyboard: true,
		ForceReply:          false,
		InlineKeyboard:      MakeBtns(prefix, btns),
	})
}

func SendBtnsMarkdown(to interface{}, what interface{}, prefix string, btns []string) (*tb.Message, error) {
	return SmartSendInner(to, what, WithMarkdown(), &tb.ReplyMarkup{
		OneTimeKeyboard:     true,
		ResizeReplyKeyboard: true,
		ForceReply:          false,
		InlineKeyboard:      MakeBtns(prefix, btns),
	})
}

func EditBtns(to *tb.Message, what interface{}, prefix string, btns []string) (*tb.Message, error) {
	return SmartEdit(to, what, &tb.ReplyMarkup{
		OneTimeKeyboard:     true,
		ResizeReplyKeyboard: true,
		ForceReply:          false,
		InlineKeyboard:      MakeBtns(prefix, btns),
	})
}

func EditBtnsMarkdown(to *tb.Message, what interface{}, prefix string, btns []string) (*tb.Message, error) {
	return SmartEdit(to, what, WithMarkdown(), &tb.ReplyMarkup{
		OneTimeKeyboard:     true,
		ResizeReplyKeyboard: true,
		ForceReply:          false,
		InlineKeyboard:      MakeBtns(prefix, btns),
	})
}

func SmartSend(to interface{}, what interface{}, options ...interface{}) (*tb.Message, error) {
	if len(options) == 0 {
		return SmartSendInner(to, what, &tb.SendOptions{
			DisableWebPagePreview: true,
			AllowWithoutReply:     true,
		})
	}
	return SmartSendInner(to, what, options...)
}

func MakeButtons(btns []string) [][]tb.InlineButton {
	btnsc := make([][]tb.InlineButton, 0)
	for _, row := range btns {
		btnscr := make([]tb.InlineButton, 0)
		for _, btn := range strings.Split(row, "||") {
			z := strings.SplitN(btn, "|", 2)
			unique := strings.TrimSpace(z[1])
			url := ""
			if strings.HasPrefix(z[1], "http://") || strings.HasPrefix(z[1], "https://") || strings.HasPrefix(z[1], "tg://") {
				unique = ""
				url = z[1]
			}
			btnscr = append(btnscr, tb.InlineButton{
				Unique: unique,
				URL:    url,
				Text:   z[0],
				Data:   "",
			})
		}
		btnsc = append(btnsc, btnscr)
	}
	return btnsc
}

func SmartSendWithBtns(to interface{}, what interface{}, buttons []string, options ...interface{}) (*tb.Message, error) {
	withOptions := []interface{}{}
	if len(options) == 0 {
		withOptions = append(withOptions, &tb.SendOptions{
			DisableWebPagePreview: true,
			AllowWithoutReply:     true,
		})
	} else {
		withOptions = options
	}

	if len(buttons) > 0 {
		withOptions = append(withOptions, &tb.ReplyMarkup{
			OneTimeKeyboard:     true,
			ResizeReplyKeyboard: true,
			ForceReply:          true,
			InlineKeyboard:      MakeButtons(buttons),
		})
	}
	return SmartSendInner(to, what, withOptions...)
}

func extractMessage(data []byte) (*tb.Message, error) {
	var resp struct {
		Result *tb.Message
	}
	if err := jsoniter.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func RevealSpoiler(msg *tb.Message) (*tb.Message, error) {
	var params = make(map[string]interface{})
	msgID, chatID := msg.MessageSig()
	params["chat_id"] = strconv.FormatInt(chatID, 10)
	params["reply_to_message_id"] = msgID
	params["disable_web_page_preview"] = true
	params["allow_sending_without_reply"] = true
	params["protect_content"] = true
	params["text"] = msg.Text

	for i, e := range msg.Entities {
		if e.Type == "spoiler" {
			msg.Entities[i].Type = tb.EntityStrikethrough
		}
	}
	params["entities"] = msg.Entities

	data, err := Bot.Raw("sendMessage", params)
	if err != nil {
		return nil, err
	}

	return extractMessage(data)
}

func SmartSendInner(to interface{}, what interface{}, options ...interface{}) (*tb.Message, error) {
	toType := Type(to)
	var m *tb.Message = nil
	var err error = nil
	if toType == "*telebot.Message" {
		mess, _ := to.(*tb.Message)
		m, err = Bot.Reply(mess, what, options...)
	} else if toType == "*telebot.Chat" {
		recp, _ := to.(*tb.Chat)
		if recp != nil {
			m, err = Bot.Send(recp, what, options...)
		} else {
			err = errors.New("chat is empty")
		}
	} else if toType == "*telebot.User" {
		recp, _ := to.(*tb.User)
		if recp != nil {
			m, err = Bot.Send(recp, what, options...)
		} else {
			err = errors.New("user is empty")
		}
	} else if toType == "int64" {
		recp, _ := to.(int64)
		m, err = Bot.Send(&tb.Chat{ID: recp}, what, options...)
	} else {
		err = errors.New("unknown type of message: " + toType)
	}
	if err != nil {
		DErrorE(err, "TeleBot Message Error")
	}
	return m, err
}

func GetUserName(u *tb.User) string {
	s := ""
	if u.FirstName != "" || u.LastName != "" {
		s = strings.TrimSpace(u.FirstName + " " + u.LastName)
	} else if u.Username != "" {
		s = "@" + u.Username
	}

	return s
}

func GetQuotableStr(f string) string {
	f = strings.ReplaceAll(f, "`", "'")
	f = strings.ReplaceAll(f, "[", "【")
	f = strings.ReplaceAll(f, "]", "】")
	return f
}

func GetQuotableSenderName(m *tb.Message) string {
	if m.SenderChat != nil {
		return GetQuotableChatName(m.SenderChat)
	} else {
		return GetQuotableUserName(m.Sender)
	}
}

func GetSenderLink(m *tb.Message) string {
	if m.SenderChat != nil {
		return "https://t.me/" + m.SenderChat.Username
	} else {
		return fmt.Sprintf("tg://user?id=%d", m.Sender.ID)
	}
}

func GetQuotableUserName(u *tb.User) string {
	return GetQuotableStr(GetUserName(u))
}

func GetChatName(u *tb.Chat) string {
	s := ""
	if u.FirstName != "" || u.LastName != "" {
		s = strings.TrimSpace(u.FirstName + " " + u.LastName)
	} else if u.Username != "" {
		s = "@" + u.Username
	}

	return s
}

func GetQuotableChatName(u *tb.Chat) string {
	return GetQuotableStr(GetChatName(u))
}

func UserIsInGroup(chatRepr string, userId int64) UIGStatus {
	cm, err := ChatMemberOf(chatRepr, Bot.Me.ID)
	if err != nil {
		return UIGErr
	} else if cm.Role != tb.Administrator && cm.Role != tb.Creator {
		return UIGErr
	}

	if userId == Bot.Me.ID {
		return UIGIn
	}

	cm, err = ChatMemberOf(chatRepr, userId)
	// if is admin, pass
	if cm.Anonymous || cm.Role == tb.Administrator || cm.Role == tb.Creator {
		return UIGIn
	}

	if err != nil || cm == nil {
		return UIGOut
	}
	if cm.Role == tb.Left || cm.Role == tb.Kicked {
		return UIGOut
	}
	return UIGIn
}

func ChatMemberOf(chatRepr string, userId int64) (*tb.ChatMember, error) {
	params := map[string]string{
		"chat_id": chatRepr,
		"user_id": strconv.FormatInt(userId, 10),
	}

	data, err := Bot.Raw("getChatMember", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result *tb.ChatMember
	}
	if err := jsoniter.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func Kick(chatId, userId int64) error {
	cm, err := Bot.ChatMemberOf(&tb.Chat{ID: chatId}, &tb.User{ID: userId})
	if err == nil {
		return Bot.Ban(&tb.Chat{ID: chatId}, cm)
	}
	return err
}

func KickOnce(chatId, userId int64) error {
	cm, err := Bot.ChatMemberOf(&tb.Chat{ID: chatId}, &tb.User{ID: userId})
	if err == nil {
		lazyScheduler.After(time.Second*15, memutils.LSC("unbanUser", &UnbanUserArgs{
			ChatId: chatId,
			UserId: userId,
		}))
		return Bot.Ban(&tb.Chat{ID: chatId}, cm)
	}
	return err
}

func Ban(chatId, userId int64, duration int64) error {
	cm, err := Bot.ChatMemberOf(&tb.Chat{ID: chatId}, &tb.User{ID: userId})
	if err == nil {
		cm.CanSendMessages = false
		cm.CanSendMedia = false
		cm.CanSendOther = false
		cm.CanAddPreviews = false
		cm.CanSendPolls = false
		cm.CanInviteUsers = false
		cm.CanPinMessages = false
		cm.CanChangeInfo = false

		cm.RestrictedUntil = time.Now().Unix() + duration
		return RestrictChatMember(&tb.Chat{ID: chatId}, cm)
	}
	return err
}

func Unban(chatId, userId int64, duration int64) error {
	cm, err := Bot.ChatMemberOf(&tb.Chat{ID: chatId}, &tb.User{ID: userId})
	if err == nil {
		cm.CanSendMessages = true
		cm.CanSendMedia = true
		cm.CanSendOther = true
		cm.CanAddPreviews = true
		cm.CanSendPolls = true
		cm.CanInviteUsers = true
		cm.CanPinMessages = true
		cm.CanChangeInfo = true
		return RestrictChatMember(&tb.Chat{ID: chatId}, cm)
	}
	return err
}

func RestrictChatMember(chat *tb.Chat, member *tb.ChatMember) error {
	rights, until := member.Rights, member.RestrictedUntil

	params := map[string]interface{}{
		"chat_id":     chat.Recipient(),
		"user_id":     member.User.Recipient(),
		"permissions": &map[string]bool{},
		"until_date":  strconv.FormatInt(until, 10),
	}

	data, _ := jsoniter.Marshal(rights)
	_ = jsoniter.Unmarshal(data, params["permissions"])
	_, err := Bot.Raw("restrictChatMember", params)
	return err
}

func BanChannel(chatId, channelId int64) error {
	params := map[string]interface{}{
		"chat_id":        strconv.FormatInt(chatId, 10),
		"sender_chat_id": strconv.FormatInt(channelId, 10),
	}

	_, err := Bot.Raw("banChatSenderChat", params)
	return err
}

func SetCommands() error {
	allCommands := [][]string{
		{"mycredit", "获取自己的积分"},
		{"redpacket", "用自己的积分发红包，发 N (1~100,000) 分给 K (1~100) 个人"},
		{"creditrank", "获取积分排行榜前 N 名"},
		{"lottery", "在积分排行榜前 N 名内抽出 K 名幸运儿"},
		{"transfer", "回复一个用户将自己的积分转移 N 分给 TA"},
	}
	cmds := []tb.Command{}
	for _, cmd := range allCommands {
		cmds = append(cmds, tb.Command{
			Text:        cmd[0],
			Description: cmd[1],
		})
	}
	return Bot.SetCommands(cmds)
}

func IsGroup(gid int64) bool {
	return I64In(&GROUPS, gid)
}

func IsAdmin(uid int64) bool {
	return I64In(&ADMINS, uid)
}

func IsGroupAdmin(c *tb.Chat, u *tb.User) bool {
	isGAS := IsGroupAdminMiaoKo(c, u)
	if isGAS {
		return true
	}
	return IsGroupAdminTelegram(c, u)
}

func IsGroupAdminMiaoKo(c *tb.Chat, u *tb.User) bool {
	gc := GetGroupConfig(c.ID)
	return gc != nil && gc.IsAdmin(u.ID)
}

func IsGroupAdminTelegram(c *tb.Chat, u *tb.User) bool {
	cm, _ := Bot.ChatMemberOf(c, u)
	if cm != nil && (cm.Role == tb.Administrator || cm.Role == tb.Creator) {
		return true
	}
	return false
}

func LazyDelete(m *tb.Message) {
	lazyScheduler.After(time.Second*10, memutils.LSC("deleteMessage", &DeleteMessageArgs{
		ChatId:    m.Chat.ID,
		MessageId: m.ID,
	}))
}

var GroupParserReg *regexp.Regexp
var SessionParserReg *regexp.Regexp

func ParseSessionMessage(str string) (int64, string) {
	result := GroupParserReg.FindString(str)
	session := SessionParserReg.FindString(str)
	if len(session) >= 2 {
		session = session[1 : len(session)-1]
	}
	if len(result) > 3 {
		id, _ := strconv.ParseInt(result[1:len(result)-1], 10, 64)
		return id, session
	}
	return 0, session
}

func ParseSession(m *tb.Message) (bool, int64, string) {
	if m.Chat.ID > 0 && m.IsReply() && m.ReplyTo.Sender.ID == Bot.Me.ID {
		chatId, sessionType := ParseSessionMessage(m.ReplyTo.Text)
		return true, chatId, sessionType
	}
	return false, 0, ""
}

func WithMarkdown() *tb.SendOptions {
	return &tb.SendOptions{
		ParseMode:             "Markdown",
		DisableWebPagePreview: true,
		AllowWithoutReply:     true,
	}
}

func init() {
	GroupParserReg = regexp.MustCompile(`\(\-\d+\)`)
	SessionParserReg = regexp.MustCompile(`\{[a-zA-Z]+\}`)
}
