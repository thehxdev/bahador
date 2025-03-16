package main

import (
	"context"
	"log"
	"os"

	"github.com/thehxdev/bahador/db"
	"github.com/thehxdev/bahador/utils"
	"github.com/thehxdev/telbot"
)

const (
	tokenEnvVar  string = "BAHADOR_BOT_TOKEN"
	hostEnvVar   string = "BAHADOR_BOT_HOST"
	dbPathEnvVar string = "BAHADOR_DB_PATH"
)

type App struct {
	Bot *telbot.Bot
	DB  *db.DB
	Log *log.Logger
}

func AppNew() (*App, error) {
	createNewDB := false
	databasePath := utils.GetNonEmptyEnv(dbPathEnvVar)
	if _, err := os.Open(databasePath); err != nil {
		createNewDB = os.IsNotExist(err)
	}
	db, err := db.New(databasePath)
	if err != nil {
		panic(err)
	}
	if createNewDB {
		db.Log.Println("creating new databse:", databasePath)
		if err := db.Setup(dbSchemaPath); err != nil {
			panic(err)
		}
	}

	a := &App{
		DB:  db,
		Log: log.New(os.Stderr, "[bahador] ", log.Ldate|log.Lshortfile),
	}

	return a, nil
}

func (app *App) InitBot(ctx context.Context) error {
	botHost := utils.GetNonEmptyEnv(hostEnvVar)
	bot, err := telbot.New(utils.GetNonEmptyEnv(tokenEnvVar), botHost)
	if err != nil {
		return err
	}
	app.Bot = bot
	app.Log.Println("telbot api host:", botHost)
	return nil
}
