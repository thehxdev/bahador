package main

// TODO: Download files bigger than maxFileLimit to /tmp and break that in smaller parts with `rar` command.
// upload all parts to bot, get download link for each file and send all links to user.

// TODO: Option to stop download/upload process by user.

// TODO: Implement an admin user to control bot and register/unregister other users.

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/joho/godotenv"
	dbpkg "github.com/thehxdev/bahador/db"
	"github.com/thehxdev/bahador/utils"
	"github.com/thehxdev/telbot"
	conv "github.com/thehxdev/telbot/ext/conversation"
)

const (
	getUpdatesLimit     int    = 100
	getUpdatesTimeout   int    = 30
	defaultDBSchemaPath string = "dbschema.sql"
)

var dbSchemaPath string = "dbschema.sql"

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}
	_ = os.Setenv("GOGC", "50")

	flag.StringVar(&dbSchemaPath, "dbschema", defaultDBSchemaPath, "path to a file that defines database schema")
	addUser := flag.Int("add-user", -1, "add a new user to database")
	flag.Parse()

	appCtx, appCancel := context.WithCancel(context.Background())
	app, err := AppNew(appCtx)
	utils.MustBeNil(err)
	db := app.DB

	if *addUser > 0 {
		utils.MustBeNil(db.UserInsert(dbpkg.User{
			UserId:  *addUser,
			IsAdmin: false,
		}))
		return
	}

	utils.MustBeNil(app.InitBot(appCtx))
	bot := app.Bot

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		go func() {
			// force exit
			<-sigChan
			os.Exit(1)
		}()
		app.Log.Println("shutting down")
		if err := db.Close(); err != nil {
			db.Log.Println(err)
		}
		appCancel()
	}()

	updatesChan, err := bot.StartPolling(appCtx, telbot.UpdateParams{
		Offset:         0,
		Limit:          getUpdatesLimit,
		Timeout:        getUpdatesTimeout,
		AllowedUpdates: []string{"message"},
	})
	if err != nil {
		app.Log.Fatal(err)
	}

	uploadWithAuthHandler := app.ConvAuthMiddleware(app.UploadCommandHandler)

	go func() {
		app.Log.Println("polling updates")
		for update := range updatesChan {
			if !updateIsValid(update) {
				continue
			}

			go func() {
				var err error
				if update.Message.IsCommand() {
					text := update.Message.Text
					if strings.HasPrefix(text, "/cancel") {
						err = app.JobCancelHandler(update)
					} else {
						switch text {
						case "/start":
							err = app.StartHandler(update)
						case "/self":
							err = app.SelfHandler(update)
						case "/up":
							conv.Start(uploadWithAuthHandler, update)
						default:
						}
					}
				} else {
					if conv.HasConversation(update.ChatId(), update.UserId()) {
						err = conv.CallNext(update)
					}
				}
				if err != nil {
					app.Log.Println(err)
				}
			}()
		}
	}()

	<-appCtx.Done()
}

func updateIsValid(update telbot.Update) bool {
	return update.Message != nil && update.Message.Text != "" && update.ChatType() == telbot.ChatTypePrivate
}
