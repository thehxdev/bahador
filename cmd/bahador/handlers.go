package main

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/thehxdev/bahador/telbot"
)

func (app *App) UploadFile(b *telbot.Bot, path string) (*telbot.Message, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fr := telbot.FileReader{
		FileName: filepath.Base(path),
		Reader:   file,
		Kind:     "document",
	}

	msg, err := b.UploadFile(telbot.UploadParams{ChatId: b.Self.Id}, fr)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (app *App) StartHandler(bot *telbot.Bot, update *telbot.Update) {
	_, err := bot.SendMessage(telbot.TextMessageParams{
		ChatId: update.Message.Chat.Id,
		Text:   "Hello, World!",
	})
	if err != nil {
		bot.Log.Println(err)
	}
}

func (app *App) SelfHandler(bot *telbot.Bot, update *telbot.Update) {
	_, err := bot.SendMessage(telbot.TextMessageParams{
		ChatId: update.Message.Chat.Id,
		Text:   strconv.Itoa(update.Message.From.Id),
	})
	if err != nil {
		bot.Log.Println(err)
	}
}

func (app *App) EchoHandler(bot *telbot.Bot, update *telbot.Update) {
	_, err := bot.SendMessage(telbot.TextMessageParams{
		ChatId: update.Message.Chat.Id,
		Text:   update.Message.Text,
	})
	if err != nil {
		bot.Log.Println(err)
	}
}
