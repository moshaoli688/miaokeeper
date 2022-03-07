package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

var VersionArg = false
var PingArg = false
var CleanArg = false
var setadmin = int64(0)
var redisServer = ""

func main() {
	if VersionArg {
		fmt.Println(version)
		os.Exit(0)
	}

	if PingArg {
		InitTelegram()
		t := time.Now().UnixMilli()
		Bot.Commands()
		// resp, _ := Bot.Raw("getMe", nil)
		t1 := time.Now().UnixMilli() - t
		Bot.Commands()
		// _, _ = Bot.Raw("getMe", nil)
		t2 := time.Now().UnixMilli() - t - t1
		Bot.Commands()
		// _, _ = Bot.Raw("getMe", nil)
		t3 := time.Now().UnixMilli() - t - t1 - t2
		fmt.Printf("Response Time: %dms, %dms, %dms (avg: %dms)\n", t1, t2, t3, (t1+t2+t3)/3)
		os.Exit(0)
	}

	if err := InitDatabase(); err != nil {
		DErrorE(err, "Database Error | Cannot initialize database.")
		os.Exit(1)
	}
	DInfo("System | Database is initialzed.")

	ReadConfigs()
	DInfo("System | Config is initialzed.")

	if setadmin > 0 {
		UpdateAdmin(setadmin, UMSet)
		os.Exit(0)
	}

	InitTelegram()
	if APIBind > 0 {
		InitRESTServer(APIBind)
	}
	if APISeed != "" {
		DLog("System | Applying API Seed: " + APISeed)
	}

	<-MakeSysChan()
	DInfo("shutting down.")
}

func init() {
	flag.StringVar(&TOKEN, "token", "", "telegram bot token")
	flag.StringVar(&TELEGRAMURL, "upstream", "", "telegram upstream api url")
	flag.StringVar(&DBCONN, "database", "", "mysql or its compatible database connection URL")
	flag.StringVar(&DBPREFIX, "prefix", "MiaoKeeper", "prefix of database table name")
	flag.StringVar(&redisServer, "redis", "", "use redis to provide high availability among restarts")
	flag.BoolVar(&VerboseMode, "verbose", false, "display all logs")
	flag.BoolVar(&VersionArg, "version", false, "display current version and exit")
	flag.BoolVar(&PingArg, "ping", false, "test the round time trip between bot and telegram server")
	flag.BoolVar(&CleanArg, "clean", false, "ignore all messages from polling")
	flag.Int64Var(&setadmin, "setadmin", 0, "set admin and delete all the other existing admins")

	flag.IntVar(&APIBind, "bind", 0, "specify a point number to bind and start an api server, for example 9876")
	flag.StringVar(&APISeed, "seed", "", "specify a seed to generate token that needs to be passed in when calling api methods")

	// deprecated flags
	APIToken := ""
	flag.StringVar(&APIToken, "api-token", "", "alias: seed, specify a seed to generate token that needs to be passed in when calling api methods")

	flag.Parse()

	{
		// deprecated flags
		if APISeed == "" && APIToken != "" {
			APISeed = APIToken
		}
	}

	InitTelegramArgs()
}
