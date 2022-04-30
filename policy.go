package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bep/debounce"
	jsoniter "github.com/json-iterator/go"
	tb "gopkg.in/telebot.v3"
)

type GroupSignType = int

const (
	GST_API_SIGN GroupSignType = iota
	GST_POLICY_CALLBACK_SIGN
)

type CreditMapping struct {
	PerValidTextMessage    int64
	PerValidStickerMessage int64

	Command    int64
	Duplicated int64
	Warn       int64
	Ban        int64
	BanBouns   int64

	HourlyUpperBound int64
}

type GroupConfig struct {
	ID            int64   `fw:"readonly"`
	Admins        []int64 `fw:"-"`
	BannedForward []int64 `fw:"-"`
	MergeTo       int64

	Locale           string
	MustFollow       string `fw:"readonly"`
	MustFollowOnJoin bool   `fw:"readonly"`
	MustFollowOnMsg  bool   `fw:"readonly"`
	GroupAPISignSeed string `fw:"readonly"`
	CreditMapping    *CreditMapping

	UnderAttackMode                    bool
	AntiSpoiler                        bool
	DisableWarn                        bool
	RedPacketCaptcha                   bool
	RedPacketCaptchaFailCreditBehavior int64

	WarnKeywords []string `fw:"-"`
	BanKeywords  []string `fw:"-"`

	NameBlackListReg   []string         `fw:"-"`
	NameBlackListRegEx []*regexp.Regexp `json:"-" fw:"-"`

	CustomReply []*CustomReplyRule `fw:"-"`

	updateLock    sync.RWMutex `json:"-" fw:"-"`
	saveLock      sync.Mutex   `json:"-" fw:"-"`
	saveDebouncer func(func()) `json:"-" fw:"-"`
}

type InvokeOptions struct {
	Rule      string // unlimit | peruser | peruserinterval | peruserday
	Value     int64
	ShowError bool
	Reset     bool
	UserOnly  bool
}

type CustomReplyRule struct {
	Match   string
	MatchEx *regexp.Regexp `json:"-"`

	Name                       string
	Limit                      int
	CreditBehavior             int
	NoForceCreditBehaviorError string
	CallbackURL                string // need a X-MiaoKeeper-Sign header

	ReplyMessage string
	ReplyTo      string // message, group, private
	ReplyButtons []string
	ReplyMode    string // deleteself, deleteorigin, deleteboth
	ReplyImage   string

	InvokeOptions *InvokeOptions

	lock sync.Mutex `json:"-"`
}

func (crr *CustomReplyRule) Consume() bool {
	crr.lock.Lock()
	defer crr.lock.Unlock()

	if crr.Limit == 0 {
		return false
	} else if crr.Limit < 0 {
		return true
	} else {
		crr.Limit -= 1
		return true
	}
}

func (crr *CustomReplyRule) Resume() {
	crr.lock.Lock()
	defer crr.lock.Unlock()

	if crr.Limit >= 0 {
		crr.Limit += 1
	}
}

func BuilRuleMessage(s string, m *tb.Message) string {
	s = strings.ReplaceAll(s, "{{ChatName}}", GetQuotableChatName(m.Chat))
	s = strings.ReplaceAll(s, "{{UserName}}", GetQuotableSenderName(m))
	s = strings.ReplaceAll(s, "{{UserLink}}", GetSenderLink(m))
	s = strings.ReplaceAll(s, "{{UserID}}", fmt.Sprintf("%d", m.Sender.ID))

	return s
}

func BuildRuleMessages(ss []string, m *tb.Message) []string {
	res := []string{}
	for _, s := range ss {
		res = append(res, BuilRuleMessage(s, m))
	}
	return res
}

func (crr *CustomReplyRule) ToJson(indent bool) (s string) {
	crr.lock.Lock()
	defer crr.lock.Unlock()

	if !indent {
		s, _ = jsoniter.MarshalToString(crr)
	} else {
		if b, err := jsoniter.MarshalIndent(crr, "", "  "); err == nil && b != nil {
			s = string(b)
		}
	}
	return
}

func NewGroupConfig(groupId int64) *GroupConfig {
	return SetGroupConfig(groupId, (&GroupConfig{
		ID: groupId,
	}).Check())
}

func (gc *GroupConfig) Check() *GroupConfig {
	if gc == nil {
		gc = &GroupConfig{}
	}

	gc.updateLock.Lock()
	defer gc.updateLock.Unlock()

	if gc.Admins == nil {
		gc.Admins = make([]int64, 0)
	}
	if gc.BannedForward == nil {
		gc.BannedForward = make([]int64, 0)
	}
	if gc.WarnKeywords == nil {
		gc.WarnKeywords = make([]string, 0)
	}
	if gc.BanKeywords == nil {
		gc.BanKeywords = make([]string, 0)
	}
	if gc.NameBlackListReg == nil {
		gc.NameBlackListReg = make([]string, 0)
	}
	if gc.NameBlackListRegEx == nil {
		gc.NameBlackListRegEx = make([]*regexp.Regexp, 0)
		for _, regStr := range gc.NameBlackListReg {
			if regex, err := regexp.Compile(regStr); regex != nil && err == nil {
				gc.NameBlackListRegEx = append(gc.NameBlackListRegEx, regex)
			} else if err != nil {
				DErrorf("Name BlackList Error | Not compilable regex=%s err=%s", regStr, err.Error())
			}
		}
	}
	if gc.CustomReply == nil {
		gc.CustomReply = make([]*CustomReplyRule, 0)
	}
	for _, crr := range gc.CustomReply {
		if regex, err := regexp.Compile(crr.Match); regex != nil && err == nil {
			crr.MatchEx = regex
		} else if err != nil {
			DErrorf("Custom Reply Error | Not compilable regex=%s err=%s", crr.Match, err.Error())
			crr.MatchEx = nil
		}
		if crr.ReplyButtons == nil {
			crr.ReplyButtons = make([]string, 0)
		}
		if crr.InvokeOptions == nil {
			crr.InvokeOptions = &InvokeOptions{
				Rule:      "unlimit",
				Value:     0,
				ShowError: true,
				Reset:     false,
				UserOnly:  false,
			}
		}
	}

	if gc.CreditMapping == nil {
		gc.CreditMapping = NewDefaultCreditMapping()
	} else {
		// should <= 0
		gc.CreditMapping.Command = MinInt64(gc.CreditMapping.Command, 0)
		gc.CreditMapping.Duplicated = MinInt64(gc.CreditMapping.Duplicated, 0)
		gc.CreditMapping.Warn = MinInt64(gc.CreditMapping.Warn, 0)
		gc.CreditMapping.Ban = MinInt64(gc.CreditMapping.Ban, 0)

		// should >= 0
		gc.CreditMapping.PerValidTextMessage = MaxInt64(gc.CreditMapping.PerValidTextMessage, 0)
		gc.CreditMapping.PerValidStickerMessage = MaxInt64(gc.CreditMapping.PerValidStickerMessage, 0)
		gc.CreditMapping.BanBouns = MaxInt64(gc.CreditMapping.BanBouns, 0)

		gc.CreditMapping.HourlyUpperBound = MaxInt64(gc.CreditMapping.HourlyUpperBound, 1)
	}

	return gc
}

func (gc *GroupConfig) ResetRules() {
	if gc != nil {
		for _, rule := range gc.CustomReply {
			if rule.InvokeOptions != nil && rule.InvokeOptions.Reset {
				key := fmt.Sprintf("%d-%s:", gc.ID, rule.Name)
				rulemap.WipePrefix(key)
			}
		}
	}
}

func (gc *GroupConfig) GenerateSign(signType GroupSignType) string {
	return SignGroup(gc.ID, signType, gc.GroupAPISignSeed)
}

func (gc *GroupConfig) POSTWithSign(url string, payload []byte, timeout time.Duration) []byte {
	return POSTJsonWithSign(url, gc.GenerateSign(GST_POLICY_CALLBACK_SIGN), payload, timeout)
}

func (gc *GroupConfig) ToJson(indent bool) (s string) {
	gc.updateLock.RLock()
	defer gc.updateLock.RUnlock()

	if !indent {
		s, _ = jsoniter.MarshalToString(gc)
	} else {
		if b, err := jsoniter.MarshalIndent(gc, "", "  "); err == nil && b != nil {
			s = string(b)
		}
	}
	return
}

func (gc *GroupConfig) FromJson(s string) error {
	gc.updateLock.Lock()
	defer gc.updateLock.Unlock()

	return jsoniter.UnmarshalFromString(s, gc)
}

func (gc *GroupConfig) Clone() *GroupConfig {
	newGC := GroupConfig{}
	newGC.FromJson(gc.ToJson(false))

	return (&newGC).Check()
}

func (gc *GroupConfig) UpdateAdmin(userId int64, method UpdateMethod) bool {
	gc.updateLock.Lock()
	defer gc.updateLock.Unlock()

	changed := false
	if method == UMSet {
		if len(gc.Admins) != 1 || gc.Admins[0] != userId {
			changed = true
			gc.Admins = []int64{userId}
		}
	} else if method == UMAdd {
		gc.Admins, changed = AddIntoInt64Arr(gc.Admins, userId)
	} else if method == UMDel {
		gc.Admins, changed = DelFromInt64Arr(gc.Admins, userId)
	}
	gc.Save()
	return changed
}

func (gc *GroupConfig) UpdateBannedForward(id int64, method UpdateMethod) bool {
	gc.updateLock.Lock()
	defer gc.updateLock.Unlock()

	changed := false
	if method == UMSet {
		if len(gc.BannedForward) != 1 || gc.BannedForward[0] != id {
			changed = true
			gc.BannedForward = []int64{id}
		}
	} else if method == UMAdd {
		gc.BannedForward, changed = AddIntoInt64Arr(gc.BannedForward, id)
	} else if method == UMDel {
		gc.BannedForward, changed = DelFromInt64Arr(gc.BannedForward, id)
	}
	gc.Save()
	return changed
}

func (gc *GroupConfig) IsAdmin(userId int64) bool {
	gc.updateLock.RLock()
	defer gc.updateLock.RUnlock()

	return I64In(&gc.Admins, userId)
}

func (gc *GroupConfig) IsBannedForward(id int64) bool {
	gc.updateLock.RLock()
	defer gc.updateLock.RUnlock()

	return I64In(&gc.BannedForward, id)
}

func (gc *GroupConfig) IsBanKeyword(m *tb.Message) bool {
	gc.updateLock.RLock()
	defer gc.updateLock.RUnlock()

	keywords := gc.BanKeywords
	if len(keywords) == 0 {
		keywords = DefaultBanKeywords
	}
	return ContainsString(keywords, m.Text)
}

func (gc *GroupConfig) IsWarnKeyword(m *tb.Message) bool {
	gc.updateLock.RLock()
	defer gc.updateLock.RUnlock()

	keywords := gc.WarnKeywords
	if len(keywords) == 0 {
		keywords = DefaultWarnKeywords
	}
	return ContainsString(keywords, m.Text)
}

func (gc *GroupConfig) IsBlackListName(u *tb.User) bool {
	gc.updateLock.RLock()
	defer gc.updateLock.RUnlock()

	namePattern := u.LastName + u.FirstName + u.LastName + u.Username
	for _, regex := range gc.NameBlackListRegEx {
		if regex.MatchString(namePattern) {
			return true
		}
	}
	return false
}

func (gc *GroupConfig) Save() {
	gc.saveLock.Lock()
	defer gc.saveLock.Unlock()

	if gc.saveDebouncer == nil {
		gc.saveDebouncer = debounce.New(time.Second)
	}

	gc.saveDebouncer(func() {
		SetGroupConfig(gc.ID, gc)
	})
}

func (gc *GroupConfig) TestCustomReplyRule(m *tb.Message) *CustomReplyRule {
	if gc == nil {
		return nil
	}

	for _, rule := range gc.CustomReply {
		if rule != nil && rule.MatchEx != nil && rule.MatchEx.MatchString(m.Text+m.Caption) && rule.Consume() {
			if rule.Limit >= 0 {
				gc.Save()
			}
			return rule
		}
	}

	return nil
}

func (gc *GroupConfig) ExecPolicy(m *tb.Message) bool {
	if rule := gc.TestCustomReplyRule(m); rule != nil {
		var sent *tb.Message
		var sendingErr error
		defer func() {
			if sent != nil && sendingErr == nil && rule.ReplyMode == "deleteboth" || rule.ReplyMode == "deleteself" {
				LazyDelete(sent)
			}
			if rule.ReplyMode == "deleteboth" || rule.ReplyMode == "deleteorigin" {
				LazyDelete(m)
			}
			if sendingErr != nil {
				SmartSendDelete(m, Locale("system.notsend", GetSenderLocale(m))+"\n\n"+sendingErr.Error())
			}
		}()

		if rule.InvokeOptions != nil {
			if rule.InvokeOptions.UserOnly && !ValidUser(m.Sender) {
				return false
			}
			key := fmt.Sprintf("%d-%s:%d", gc.ID, rule.Name, m.Sender.ID)
			switch rule.InvokeOptions.Rule {
			case "peruser":
				if rulemap.Add(key) > int(rule.InvokeOptions.Value) {
					if rule.InvokeOptions.ShowError {
						SmartSendDelete(m, Locale("policy.rule.limit.peruser", GetSenderLocale(m)))
					}
					return false
				}
			case "peruserinterval":
				if rulemap.Add(key) > 1 {
					if rule.InvokeOptions.ShowError {
						SmartSendDelete(m, Locale("policy.rule.limit.peruserinterval", GetSenderLocale(m)))
					}
					return false
				} else {
					rulemap.SetExpire(key, time.Duration(rule.InvokeOptions.Value)*time.Second)
				}
			case "peruserday":
				key += fmt.Sprintf(":%d", time.Now().Day())
				if rulemap.Add(key) > int(rule.InvokeOptions.Value) {
					if rule.InvokeOptions.ShowError {
						SmartSendDelete(m, Locale("policy.rule.limit.peruserday", GetSenderLocale(m)))
					}
					return false
				}
			}
		}

		if rule.CreditBehavior != 0 {
			if ci := GetCreditInfo(gc.ID, m.Sender.ID); ci != nil {
				abort := false
				ci.Acquire(func() {
					if rule.NoForceCreditBehaviorError != "" && rule.CreditBehavior < 0 {
						if ci.Credit+int64(rule.CreditBehavior) < 0 {
							// cannot substract credit points
							textMessage := BuilRuleMessage(rule.NoForceCreditBehaviorError, m)
							SmartSendDelete(m, textMessage, WithMarkdown())
							rule.Resume()
							gc.Save()
							abort = true

							return
						}
					}
					ci.unsafeUpdate(UMAdd, int64(rule.CreditBehavior), (&UserInfo{}).From(m.Chat.ID, m.Sender), OPByPolicy, ci.ID, fmt.Sprintf("ByPolicy:%.8s", rule.Name))
				})
				if abort {
					return true
				}
			}
		}

		if rule.ReplyMessage != "" {
			var target interface{} = m
			if rule.ReplyTo == "group" {
				target = m.Chat
			} else if rule.ReplyTo == "private" {
				target = m.Sender
			}

			textMessage := BuilRuleMessage(rule.ReplyMessage, m)
			var message interface{} = textMessage
			if rule.ReplyImage != "" {
				if u, err := url.Parse(rule.ReplyImage); err == nil && u != nil {
					message = &tb.Photo{
						File:    tb.FromURL(rule.ReplyImage),
						Caption: textMessage,
					}
				}
			}

			sent, sendingErr = SmartSendWithBtns(target, message, BuildRuleMessages(rule.ReplyButtons, m), WithMarkdown())
		}

		if rule.CallbackURL != "" {
			realURL := BuilRuleMessage(rule.CallbackURL, m)
			if strings.HasPrefix(realURL, "tg://") {
				realURL = strings.Replace(realURL, "tg://", "https://api.telegram.org/bot"+Bot.Token+"/", 1)
			}
			if u, err := url.Parse(realURL); err == nil && u != nil {
				go gc.POSTWithSign(realURL, []byte(rule.ToJson(false)), time.Second*3)
			}
		}

		return true
	}

	return false
}

func NewDefaultCreditMapping() *CreditMapping {
	return &CreditMapping{
		PerValidTextMessage:    1,
		PerValidStickerMessage: 1,

		Command:    -5,
		Duplicated: -5,
		Warn:       -25,
		Ban:        -50,
		BanBouns:   15,

		HourlyUpperBound: 20,
	}
}
