package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/thehxdev/telbot"
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

	msg, err := b.UploadFile(context.Background(), telbot.UploadParams{ChatId: b.Self.Id}, fr)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (app *App) StartHandler(bot *telbot.Bot, update *telbot.Update) error {
	_, err := bot.SendMessage(telbot.TextMessageParams{
		ChatId: update.Message.Chat.Id,
		Text:   "Hello, World!",
	})
	return err
}

func (app *App) SelfHandler(bot *telbot.Bot, update *telbot.Update) error {
	_, err := bot.SendMessage(telbot.TextMessageParams{
		ChatId: update.Message.Chat.Id,
		Text:   strconv.Itoa(update.Message.From.Id),
	})
	return err
}

func (app *App) EchoHandler(bot *telbot.Bot, update *telbot.Update) error {
	_, err := bot.SendMessage(telbot.TextMessageParams{
		ChatId: update.Message.Chat.Id,
		Text:   update.Message.Text,
	})
	return err
}
