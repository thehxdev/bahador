package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/joho/godotenv"
	dbpkg "github.com/thehxdev/bahador/db"
	"github.com/thehxdev/bahador/utils"
	"github.com/thehxdev/telbot"
	conv "github.com/thehxdev/telbot/ext/conversation"
)

const (
	updatesLimit        int    = 100
	updatesTimeout      int    = 30
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
		app.Bot.Shutdown()
		appCancel()
	}()

	updatesChan, err := bot.StartPolling(telbot.UpdateParams{
		Offset:         0,
		Timeout:        updatesTimeout,
		Limit:          updatesLimit,
		AllowedUpdates: []string{"message"},
	})

	uploadCommandAuth := app.AuthMiddleware(app.UploadCommandHandler)
	getLinksConv, _ := conv.New([]telbot.UpdateHandlerFunc{uploadCommandAuth, app.LinksMessageHandler})

	go func() {
		app.Log.Println("polling updates")
		for update := range updatesChan {
			if update.Message == nil || update.Message.Text == "" || update.ChatType() != telbot.ChatTypePrivate {
				continue
			}
			go func(update telbot.Update) {
				var err error
				switch update.Message.Text {
				case "/start":
					err = app.StartHandler(update)
				case "/self":
					err = app.SelfHandler(update)
				case "/up":
					getLinksConv.Start(update)
				default:
					if conv.HasConversation(update.ChatId(), update.UserId()) {
						err = conv.HandleUpdate(update)
					}
				}
				if err != nil {
					app.Log.Println(err)
				}
			}(update)
		}
	}()

	<-appCtx.Done()
}
