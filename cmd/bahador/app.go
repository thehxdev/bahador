package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/thehxdev/bahador/db"
	"github.com/thehxdev/bahador/utils"
	"github.com/thehxdev/telbot"
	"github.com/thehxdev/telbot/types"
)

const (
	maxFileSize  int    = 500 * 1024 * 1024
	workersCount int    = 5
	tokenEnvVar  string = "BAHADOR_BOT_TOKEN"
	hostEnvVar   string = "BAHADOR_BOT_HOST"
	dbPathEnvVar string = "BAHADOR_DB_PATH"
	dlPathEnvVar string = "BAHADOR_DL_PATH"
	urlRegex     string = `https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`
)

type dlResult struct {
	error
	msg *types.Message
}

type dlJob struct {
	url string
	resChan chan dlResult
}

type dlFile struct {
	name string
	size int
}

type App struct {
	Bot *telbot.Bot
	DB  *db.DB
	Log *log.Logger

	jobChan chan dlJob
}

func AppNew(ctx context.Context) (*App, error) {
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
		jobChan: make(chan dlJob, workersCount),
	}

	for range workersCount {
		go a.worker(ctx)
	}

	return a, nil
}

func (app *App) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job := <-app.jobChan
		func() {
			res := dlResult{}
			defer func() { job.resChan <- res }()

			dlFile, err := app.downloadAndSaveFile(ctx, job.url)
			if err != nil {
				res.error = err
				return
			}
			defer os.Remove(dlFile.name)

			app.Log.Println("file downloaded:", dlFile.name)
			app.Log.Println("uploading file:", dlFile.name)

			msg, err := app.UploadFile(dlFile.name, dlFile.name)
			if err != nil {
				res.error = err
				return
			}
			res.msg = msg
		}()
	}
}

func getFilename(resp *http.Response) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		_, fname, found := strings.Cut(cd, "filename=")
		if !found {
			return ""
		}
		return strings.Fields(fname)[0]
	}
	return path.Base(resp.Request.URL.Path)
}

func getRemoteFileInfo(ctx context.Context, fileUrl string) (fname string, fsize int, err error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", fileUrl, nil)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	fsize, err = strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return
	}

	fname = getFilename(resp)
	return
}

func (app *App) downloadAndSaveFile(ctx context.Context, fileUrl string) (*dlFile, error) {
	fname, fsize, err := getRemoteFileInfo(ctx, fileUrl)
	if err != nil {
		return nil, err
	}
	if fname == "" {
		return nil, errors.New("could not get file name")
	}
	if fsize > maxFileSize {
		return nil, errors.New("file size is bigger than max file size")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fileUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	app.Log.Printf("downloading file (%dKB): %s\n", fsize/1024, fname)
	f, err := os.Create(fname)
	if err != nil {
		return nil, err
	}

	ioCtx, ioCancel := context.WithTimeout(ctx, time.Minute * 60)
	defer ioCancel()

	done := make(chan error)
	go func() {
		var n   int64
		var err error
		n, err = io.Copy(f, resp.Body)
		if n != int64(fsize) {
			err = errors.New("file does not downloaded properly")
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
		return &dlFile{name: fname, size: fsize}, nil
	case <-ctx.Done():
		return nil, ioCtx.Err()
	}
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
