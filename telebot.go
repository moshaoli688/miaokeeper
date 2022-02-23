package main

import (
	"math/rand"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/BBAlliance/miaokeeper/memutils"
	"github.com/bep/debounce"
	tb "gopkg.in/tucnak/telebot.v2"
)

type UIGStatus int

const (
	UIGIn UIGStatus = iota
	UIGOut
	UIGErr
)

var Bot *tb.Bot
var TOKEN = ""
var TELEGRAMURL = ""

var APIBind = 0
var APISeed = ""

var GROUPS = []int64{}
var ADMINS = []int64{}

var lastID = int64(-1)
var lastText = ""
var puncReg *regexp.Regexp

var zcomap *ObliviousMapInt
var creditomap *ObliviousMapInt
var votemap *ObliviousMapInt

var joinmap *ObliviousMapInt

var redpacketrankmap *ObliviousMapStr
var redpacketmap *ObliviousMapInt
var redpacketnmap *ObliviousMapInt
var redpacketcaptcha *ObliviousMapStr

var debouncer func(func())
var lazyScheduler *memutils.LazyScheduler

var usercreditlock sync.Mutex

var DefaultWarnKeywords = []string{"口臭", "口 臭", "口  臭", "口臭!", "口臭！", "嘴臭", "嘴 臭", "嘴  臭", "嘴臭!", "嘴臭！"}
var DefaultBanKeywords = []string{"恶意广告", "惡意廣告", "恶意发言", "惡意發言", "恶意举报", "惡意舉報", "惡意檢舉", "恶意嘴臭", "恶意口臭"}

func InitTelegram() {
	var err error
	Bot, err = tb.NewBot(tb.Settings{
		Token: TOKEN,
		Poller: &tb.LongPoller{
			Timeout: 10 * time.Second,
			AllowedUpdates: []string{
				"message",
				"edited_message",
				// "channel_post",
				// "edited_channel_post",
				// "inline_query",
				// "chosen_inline_result",
				"callback_query",
				// "shipping_query",
				// "pre_checkout_query",
				// "poll",
				// "poll_answer",
				"my_chat_member",
				"chat_member",
				// "chat_join_request",
			},
		},
		URL: TELEGRAMURL,
	})

	if err != nil {
		DErrorf("TeleBot Error | cannot initialize bot | err=%s", err.Error())
		os.Exit(1)
	}

	if Bot == nil {
		DErrorf("TeleBot Error | cannot initialize bot | err=Unknown error")
		os.Exit(1)
	}

	if !PingArg && !CleanArg {

		if !Bot.Me.CanJoinGroups {
			DErrorf("TeleBot Error | bot cannot be added to group, please check with @botfather")
			os.Exit(1)
		}

		if !Bot.Me.CanReadMessages {
			DErrorf("TeleBot Error | bot cannot be run under privacy mode, please check with @botfather")
			os.Exit(1)
		}

		if err != nil {
			DErrorE(err, "TeleBot Error | cannot initialize telegram bot")
			os.Exit(1)
		}

		err = SetCommands()
		if err != nil {
			DErrorE(err, "TeleBot Error | cannot update commands for telegram bot")
		}

		// ---------------- Super Admin ----------------

		Bot.Handle("/su_export_credit", CmdSuExportCredit)
		Bot.Handle("/su_add_group", CmdSuAddGroup)
		Bot.Handle("/su_del_group", CmdSuDelGroup)
		Bot.Handle("/su_add_admin", CmdSuAddAdmin)
		Bot.Handle("/su_del_admin", CmdSuDelAdmin)
		Bot.Handle("/su_quit_group", CmdSuQuitGroup)

		// ---------------- Group Admin ----------------

		Bot.Handle("/add_admin", CmdAddAdmin)
		Bot.Handle("/del_admin", CmdDelAdmin)
		Bot.Handle("/import_policy", CmdSetPolicy)
		Bot.Handle("/export_policy", CmdGetPolicy)
		Bot.Handle("/export_token", CmdGetToken)
		Bot.Handle("/ban_forward", CmdBanForward)
		Bot.Handle("/unban_forward", CmdUnbanForward)
		Bot.Handle("/set_credit", CmdSetCredit)
		Bot.Handle("/add_credit", CmdAddCredit)
		Bot.Handle("/check_credit", CmdCheckCredit)
		Bot.Handle("/set_antispoiler", CmdSetAntiSpoiler)
		Bot.Handle("/set_channel", CmdSetChannel)
		Bot.Handle("/set_locale", CmdSetLocale)
		Bot.Handle("/create_lottery", CmdCreateLottery)

		Bot.Handle("/creditrank", CmdCreditRank)
		Bot.Handle("/creditlog", CmdCreditLog)
		Bot.Handle("/redpacket", CmdRedpacket)
		Bot.Handle("/lottery", CmdLottery)
		Bot.Handle("/transfer", CmdCreditTransfer)

		// ---------------- Normal User ----------------

		Bot.Handle("/ban_user", CmdBanUserCommand)
		Bot.Handle("/unban_user", CmdUnbanUserCommand)
		Bot.Handle("/kick_user", CmdKickUserCommand)

		Bot.Handle("/mycredit", CmdMyCredit)
		Bot.Handle("/version", CmdVersion)
		Bot.Handle("/id", CmdID)
		Bot.Handle("/ping", CmdPing)

		Bot.Handle(tb.OnUserLeft, CmdOnUserLeft)
		Bot.Handle(tb.OnChatMember, CmdOnChatMember)
		Bot.Handle(tb.OnUserJoined, CmdOnUserJoined)
		Bot.Handle(tb.OnPinned, CmdOnPinned)

		Bot.Handle(tb.OnCallback, CmdOnCallback)
		Bot.Handle(tb.OnPhoto, CmdOnMisc)
		Bot.Handle(tb.OnAnimation, CmdOnMisc)
		Bot.Handle(tb.OnVideo, CmdOnMisc)
		Bot.Handle(tb.OnEdited, CmdOnMisc)
		Bot.Handle(tb.OnDocument, CmdOnDocument)

		Bot.Handle(tb.OnText, CmdOnText)
		Bot.Handle(tb.OnSticker, CmdOnSticker)

		InitCallback()
	}

	go Bot.Start()

	if !PingArg {
		DInfo("System | MiaoKeeper bot is up.")
		lazyScheduler.Recover()
	}

	if CleanArg {
		DInfo("System | Clean mode is on.")
	}
}

func InitTelegramArgs() {
	rand.Seed(time.Now().UnixNano())
	puncReg = regexp.MustCompile(`^[!$%&"'*+,\-.{}[\]():;=?^_|~\\][a-zA-Z0-9]+`)
	// puncReg = regexp.MustCompile(`^[!"#$%&'()*+,\-./:;<=>?@[\]^_{|}~\\` + "`" + `][a-zA-Z0-9]+`)

	// create a memory cache driver
	var memdriver memutils.MemDriver
	if redisServer != "" {
		args := strings.SplitN(redisServer, "@", 2)
		if len(args) < 2 {
			args = append(args, "")
			args[0], args[1] = args[1], args[0]
		}
		memdriver = &memutils.MemDriverRedis{}
		memdriver.Init(args[1], args[0])
	}

	if memdriver == nil {
		memdriver = &memutils.MemDriverMemory{}
		memdriver.Init()
	}

	zcomap = NewOMapInt("zc/", time.Hour, true, memdriver)
	creditomap = NewOMapInt("credit/", time.Hour, false, memdriver)
	votemap = NewOMapInt("vote/", time.Minute*30, false, memdriver)
	joinmap = NewOMapInt("join/", time.Minute*5+time.Second*30, false, memdriver)

	redpacketrankmap = NewOMapStr("rprank/", time.Hour*24, false, memdriver)
	redpacketmap = NewOMapInt("rp/", time.Hour*24, false, memdriver)
	redpacketnmap = NewOMapInt("rpname/", time.Hour*24, false, memdriver)
	redpacketcaptcha = NewOMapStr("rpcaptcha/", time.Hour*24, false, memdriver)

	debouncer = debounce.New(time.Second)
	lazyScheduler = memutils.NewLazyScheduler(memdriver)

	InitScheduler()
}
