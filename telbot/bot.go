// A simple telegram bot api library that supports a little subset of official telegram api
// for building small and simple bots. Heavily inspired by `https://github.com/go-telegram-bot-api/telegram-bot-api`

package telbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const GetUpdatesSleepTime = time.Second * 1

type Bot struct {
	Token       string
	BaseUrl     string
	BaseFileUrl string
	Self        *User
	UpdatesChan chan Update
	HttpClient  *http.Client
	Ctx         context.Context
	Log         *log.Logger

	// Shutdown signal channel
	sdChan chan struct{}
}

type UpdateHandlerFunc func(bot *Bot, update *Update)

type APIResponse struct {
	Ok          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}

type ParamsStringMap map[string]string

type APIParams interface {
	ToParamsStringMap() (*ParamsStringMap, error)
}

func APIParamsToUrlValues(p APIParams) (*url.Values, error) {
	values := &url.Values{}
	pMap, err := p.ToParamsStringMap()
	if err != nil {
		return nil, err
	}
	for k, v := range *pMap {
		values.Add(k, v)
	}
	return values, nil
}

func (p *ParamsStringMap) ToParamsStringMap() (*ParamsStringMap, error) {
	return p, nil
}

func (p *ParamsStringMap) AddNonEmpty(key string, value string) {
	if value != "" {
		(*p)[key] = value
	}
}

func (p *ParamsStringMap) AddNonZeroInt(key string, value int) {
	if value != 0 {
		p.AddNonEmpty(key, strconv.Itoa(value))
	}
}

func (p *ParamsStringMap) AddInterface(key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	p.AddNonEmpty(key, string(b))
	return nil
}

// Create a new instance of Bot
func New(ctx context.Context, token string, host ...string) (*Bot, error) {
	h := "api.telegram.org"
	if len(host) > 0 {
		h = host[0]
	}

	b := &Bot{
		Token:       token,
		BaseUrl:     fmt.Sprintf("https://%s/bot%s", h, token),
		BaseFileUrl: fmt.Sprintf("https://%s/file/bot%s", h, token),
		HttpClient:  &http.Client{},
		Ctx:         ctx,
		sdChan:      make(chan struct{}),
		Log:         log.New(os.Stderr, "[telbot] ", log.Ldate|log.Lshortfile),
	}

	botUser, err := b.GetMe()
	if err != nil {
		return nil, err
	}
	b.Self = botUser

	return b, nil
}

func (b *Bot) Shutdown() {
	// TODO: Handle bot graceful shutdown
	close(b.sdChan)
	close(b.UpdatesChan)
	// b.StopPolling()
}

func (b *Bot) createMethodUrl(method string) string {
	return fmt.Sprintf("%s/%s", b.BaseUrl, method)
}

func (b *Bot) SendRequest(method string, params APIParams) (*APIResponse, error) {
	var err error
	var urlValues *url.Values
	if params != nil {
		urlValues, err = APIParamsToUrlValues(params)
		if err != nil {
			return nil, err
		}
	} else {
		urlValues = &url.Values{}
	}

	reqUrl := b.createMethodUrl(method)
	req, err := http.NewRequestWithContext(b.Ctx, "POST", reqUrl, strings.NewReader(urlValues.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	apiResp := &APIResponse{}
	if err := json.NewDecoder(resp.Body).Decode(apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Ok {
		return nil, errors.New(apiResp.Description)
	}

	return apiResp, nil
}

func (b *Bot) GetMe() (*User, error) {
	u := &User{}
	apiResp, err := b.SendRequest("getMe", nil)
	if err != nil {
		return nil, err
	}

	if !apiResp.Ok {
		return nil, errors.New(apiResp.Description)
	}

	err = json.Unmarshal(apiResp.Result, u)
	return u, err
}

func (b *Bot) GetUpdates(params UpdateParams) ([]Update, error) {
	updates := []Update{}
	apiResp, err := b.SendRequest("getUpdates", &params)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(apiResp.Result, &updates)
	if err != nil {
		return nil, err
	}

	return updates, nil
}

func (b *Bot) UploadFile(params UploadParams, file FileInfo) (*Message, error) {
	r, w := io.Pipe()
	m := multipart.NewWriter(w)

	go func() {
		defer w.Close()
		defer m.Close()

		pMap, _ := params.ToParamsStringMap()
		for key, value := range *pMap {
			if err := m.WriteField(key, value); err != nil {
				w.CloseWithError(err)
				return
			}
		}

		fileName, fileReader, err := file.UploadInfo()
		if err != nil {
			w.CloseWithError(err)
			return
		}
		part, err := m.CreateFormFile(file.FileKind(), fileName)
		if err != nil {
			w.CloseWithError(err)
			return
		}

		if _, err := io.Copy(part, fileReader); err != nil {
			w.CloseWithError(err)
			return
		}

		if closer, ok := fileReader.(io.ReadCloser); ok {
			if err = closer.Close(); err != nil {
				w.CloseWithError(err)
				return
			}
		}
	}()

	reqUrl := b.createMethodUrl("sendDocument")
	req, err := http.NewRequest("POST", reqUrl, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", m.FormDataContentType())

	resp, err := b.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	apiResp := &APIResponse{}
	if err := json.NewDecoder(resp.Body).Decode(apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Ok {
		return nil, errors.New(apiResp.Description)
	}

	msg := &Message{}
	if err := json.NewDecoder(bytes.NewReader(apiResp.Result)).Decode(msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func (b *Bot) GetFile(fileId string) (*File, error) {
	apiResp, err := b.SendRequest("getFile", &ParamsStringMap{"file_id": fileId})
	if err != nil {
		return nil, err
	}

	file := &File{}
	err = json.Unmarshal(apiResp.Result, file)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (b *Bot) SendMessage(params TextMessageParams) (*Message, error) {
	apiResp, err := b.SendRequest("sendMessage", &params)
	if err != nil {
		return nil, err
	}

	msg := &Message{}
	err = json.Unmarshal(apiResp.Result, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (b *Bot) StartPolling(params UpdateParams) (<-chan Update, error) {
	b.UpdatesChan = make(chan Update, params.Limit)

	go func() {
		for {
			select {
			case <-b.sdChan:
				return
			default:
			}
			updates, err := b.GetUpdates(params)
			if err != nil {
				log.Println(err)
				time.Sleep(time.Second * 5)
				continue
			}
			for _, update := range updates {
				if update.Id >= params.Offset {
					params.Offset = update.Id + 1
					b.UpdatesChan <- update
				}
			}
			time.Sleep(GetUpdatesSleepTime)
		}
	}()

	return b.UpdatesChan, nil
}

// func (b *Bot) StopPolling() {
// 	close(b.UpdatesChan)
// }
