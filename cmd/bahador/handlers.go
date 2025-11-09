package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/thehxdev/bahador/utils"
	"github.com/thehxdev/telbot"
	conv "github.com/thehxdev/telbot/ext/conversation"
)

var httpUrlRegexp *regexp.Regexp = regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)

func (app *App) StartHandler(update telbot.Update) error {
	_, err := app.Bot.SendMessage(context.Background(), telbot.TextMessageParams{
		ChatId: update.ChatId(),
		Text:   "Hello, World!",
	})
	return err
}

func (app *App) SelfHandler(update telbot.Update) error {
	_, err := app.Bot.SendMessage(context.Background(), telbot.TextMessageParams{
		ChatId: update.ChatId(),
		Text:   strconv.Itoa(update.UserId()),
	})
	return err
}

func (app *App) UploadCommandHandler(c *conv.Conversation, update telbot.Update) error {
	_, err := app.Bot.SendMessage(context.Background(), telbot.TextMessageParams{
		ChatId: update.ChatId(),
		Text:   "Send a download link.",
	})
	c.Next = app.LinksMessageHandler
	return err
}

func (app *App) JobCancelHandler(update telbot.Update) error {
	jobIdStr, _ := strings.CutPrefix(update.Message.Text, "/cancel")
	jobId, err := strconv.ParseInt(jobIdStr, 10, 64)
	if err != nil {
		return err
	}
	var msgText string
	if cancelChan, ok := app.jobMap[jobId]; ok {
		close(cancelChan)
		delete(app.jobMap, jobId)
		msgText = fmt.Sprintf("Job %d canceled.", jobId)
	} else {
		msgText = "Job does not exist."
	}
	_, err = app.Bot.SendMessage(context.Background(), telbot.TextMessageParams{
		ChatId: update.ChatId(),
		Text:   msgText,
	})
	return err
}

func (app *App) LinksMessageHandler(c *conv.Conversation, update telbot.Update) error {
	params := telbot.TextMessageParams{ChatId: update.ChatId()}

	if update.Message.Text == "" {
		params.Text = "Your message does not contain any text data."
		app.Bot.SendMessage(context.Background(), params)
		return &conv.EndConversation{}
	}

	if !httpUrlRegexp.MatchString(update.Message.Text) {
		params.Text = "Your message does not match to a valid HTTPS URL."
		params.ReplyToMessageId = update.MessageId()
		app.Bot.SendMessage(context.Background(), params)
		return &conv.EndConversation{}
	}

	job := dlJob{
		url:        update.Message.Text,
		resChan:    make(chan jobResult, 1),
		cancelChan: make(chan struct{}, 1),
	}

	// FIXME: handle collision (same jobId with another jobId)
	jobId, _ := utils.GenRandInt64(0, 100)
	app.jobMap[jobId] = job.cancelChan

	chatId := update.ChatId()
	statSuffix := fmt.Sprintf("\n/cancel%d", jobId)

	statMsg, _ := app.Bot.SendMessage(context.Background(), telbot.TextMessageParams{
		ChatId:           chatId,
		Text:             "Processing URL..." + statSuffix,
		ReplyToMessageId: update.MessageId(),
	})

	job.eventLogger = func(format string, v ...any) {
		var logText string
		if len(v) > 0 {
			logText = fmt.Sprintf(format, v)
		} else {
			logText = fmt.Sprint(format)
		}
		app.Log.Println(logText)
		app.Bot.EditMessageText(context.Background(), telbot.EditMessageTextParams{
			ChatId:    chatId,
			MessageId: statMsg.Id,
			Text:      logText + statSuffix,
		})
	}

	app.jobChan <- job

	res := <-job.resChan
	close(job.resChan)

	select {
	case _, ok := <-job.cancelChan:
		if ok {
			close(job.cancelChan)
		}
	default:
	}

	delete(app.jobMap, jobId)

	var urls []string
	var statText string
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

	urls = []string{}
	for _, fileId := range res.fileIds {
		u, _ := url.JoinPath(app.Bot.BaseFileUrl, fileId)
		urls = append(urls, u)
	}
	statText = strings.Join(urls, "\n\n")

done:
	app.Bot.EditMessageText(context.Background(), telbot.EditMessageTextParams{
		ChatId:    chatId,
		MessageId: statMsg.Id,
		Text:      statText,
	})
	return &conv.EndConversation{}
}
