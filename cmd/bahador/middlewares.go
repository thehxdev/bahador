package main

import (
	"github.com/thehxdev/telbot"
)

func (app *App) AuthMiddleware(next telbot.UpdateHandlerFunc) telbot.UpdateHandlerFunc {
	return func(update telbot.Update) error {
		if _, err := app.DB.UserAuthenticate(update.Message.From.Id); err == nil {
			return next(update)
		}
		return nil
	}
}
