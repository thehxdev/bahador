package main

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/thehxdev/telbot"
	conv "github.com/thehxdev/telbot/ext/conversation"
	"github.com/thehxdev/telbot/types"
)

func (app *App) UploadFile(path, fname string) (*types.Message, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fr := telbot.FileReader{
		FileName: fname,
		Reader:   file,
		Kind:     "document",
	}

	uploadCtx, uploadCancel := context.WithTimeout(context.Background(), time.Hour*1)
	defer uploadCancel()

	bot := app.Bot
	msg, err := bot.UploadFile(uploadCtx, telbot.UploadParams{ChatId: bot.Self.Id, Method: "sendDocument"}, fr)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (app *App) StartHandler(update telbot.Update) error {
	_, err := update.SendMessage(telbot.TextMessageParams{
		ChatId: update.ChatId(),
		Text:   "Hello, World!",
	})
	return err
}

func (app *App) SelfHandler(update telbot.Update) error {
	_, err := update.SendMessage(telbot.TextMessageParams{
		ChatId: update.ChatId(),
		Text:   strconv.Itoa(update.UserId()),
	})
	return err
}

func (app *App) UploadCommandHandler(update telbot.Update) error {
	_, err := update.SendMessage(telbot.TextMessageParams{
		ChatId: update.ChatId(),
		Text:   "Send a download link.",
	})
	return err
}

func (app *App) LinksMessageHandler(update telbot.Update) error {
	params := telbot.TextMessageParams{ChatId: update.ChatId()}

	if update.Message.Text == "" {
		params.Text = "Your message does not contain any text data."
		update.SendMessage(params)
		return conv.EndConversation
	}

	job := dlJob{
		url:     update.Message.Text,
		resChan: make(chan dlResult, 1),
	}

	app.jobChan <- job

	statText := "Got it! Processing URL..."
	params.Text = statText
	params.ReplyToMsgId = update.MessageId()
	statMsg, _ := update.SendMessage(params)

	res := <-job.resChan
	close(job.resChan)

	if err := res.error; err != nil {
		app.Log.Println(err)
		switch err.(type) {
		case *EmptyFileNameError:
			statText = err.Error()
		case *MaxFileSizeError:
			statText = err.Error()
		case *IncompleteDownloadError:
			statText = err.Error()
		case *NonZeroStatusError:
			statText = err.Error()
		default:
			statText = "failed to download file (probably internal server error)"
		}
		goto done
	}

	statText, _ = url.JoinPath(app.Bot.BaseFileUrl, res.msg.Document.FileId)

done:
	update.EditMessage(telbot.EditMessageTextParams{
		ChatId:    statMsg.Chat.Id,
		MessageId: statMsg.Id,
		Text:      statText,
	})
	return conv.EndConversation
}
