package main

import (
	"github.com/thehxdev/bahador/telbot"
)

func (app *App) AuthMiddleware(next telbot.UpdateHandlerFunc) telbot.UpdateHandlerFunc {
	return func(bot *telbot.Bot, update *telbot.Update) {
		if _, err := app.DB.UserAuthenticate(update.Message.From.Id); err == nil {
			next(bot, update)
		}
	}
}
