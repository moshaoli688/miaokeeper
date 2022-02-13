package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bep/debounce"
	jsoniter "github.com/json-iterator/go"
	tb "gopkg.in/tucnak/telebot.v2"
)

type GroupSignType = int

const (
	GST_API_SIGN GroupSignType = iota
	GST_POLICY_CALLBACK_SIGN
)

type GroupConfig struct {
	ID            int64
	Admins        []int64
	BannedForward []int64
	MergeTo       int64

	Locale           string
	MustFollow       string
	MustFollowOnJoin bool
	MustFollowOnMsg  bool

	AntiSpoiler bool
	DisableWarn bool

	WarnKeywords []string
	BanKeywords  []string

	NameBlackListReg   []string
	NameBlackListRegEx []*regexp.Regexp `json:"-"`

	CustomReply []*CustomReplyRule

	updateLock    sync.RWMutex `json:"-"`
	saveLock      sync.Mutex   `json:"-"`
	saveDebouncer func(func()) `json:"-"`
}

type CustomReplyRule struct {
	Match   string
	MatchEx *regexp.Regexp `json:"-"`

	Name           string
	Limit          int
	CreditBehavior int
	CallbackURL    string // need a X-MiaoKeeper-Sign header

	ReplyMessage string
	ReplyTo      string // message, group, private
	ReplyButtons []string
	ReplyMode    string // deleteself, deleteorigin, deleteboth

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
	}
	return gc
}

func (gc *GroupConfig) GenerateSign(signType GroupSignType) string {
	return SignGroup(gc.ID, signType)
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
