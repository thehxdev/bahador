package main

import (
	"context"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/thehxdev/bahador/db"
	"github.com/thehxdev/bahador/utils"
	"github.com/thehxdev/telbot"
	"github.com/thehxdev/telbot/types"
)

const (
	maxFileSize  int64  = 210 * 1024 * 1024
	workersCount int    = 5
	tokenEnvVar  string = "BAHADOR_BOT_TOKEN"
	hostEnvVar   string = "BAHADOR_BOT_HOST"
	dbPathEnvVar string = "BAHADOR_DB_PATH"
	// dlPathEnvVar string = "BAHADOR_DL_PATH"
)

type dlResult struct {
	error
	msg *types.Message
}

type dlJob struct {
	url        string
	resChan    chan dlResult
	cancelChan chan struct{}
}

// type dlFile struct {
// 	name string
// 	size int64
// }

type App struct {
	Bot *telbot.Bot
	DB  *db.DB
	Log *log.Logger

	jobChan chan dlJob
	jobMap  map[int64](chan struct{})
	// jobMap  struct {
	// 	mu *sync.RWMutex
	// 	table map[int64](chan struct{})
	// }
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
		DB:      db,
		Log:     log.New(os.Stderr, "[bahador] ", log.Ldate|log.Lshortfile),
		jobChan: make(chan dlJob, workersCount),
		jobMap:  make(map[int64]chan struct{}),
	}

	for range workersCount {
		// go a.worker(ctx)
		go a.workerWithPipe(ctx)
	}

	return a, nil
}

func (app *App) workerWithPipe(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job := <-app.jobChan
		res := dlResult{}

		res.error = func() error {
			jobCtx, jobCancel := context.WithTimeout(ctx, time.Minute*30)
			defer jobCancel()

			go func() {
				<-job.cancelChan
				jobCancel()
			}()

			fname, fsize, err := getRemoteFileInfo(jobCtx, job.url)
			if err != nil {
				return err
			}
			if fname == "" {
				return ErrEmptyFileName
			}
			if fsize > maxFileSize {
				return ErrMaxFileSize
			}

			dlReq, err := http.NewRequestWithContext(jobCtx, "GET", job.url, nil)
			if err != nil {
				return err
			}

			dlResp, err := http.DefaultClient.Do(dlReq)
			if err != nil {
				return err
			}
			defer dlResp.Body.Close()

			if dlResp.StatusCode != http.StatusOK {
				return ErrNonZeroStatusCode
			}

			errChan := make(chan error, 2)
			pipeReader, pipeWriter := io.Pipe()
			defer func() {
				pipeWriter.Close()
				pipeReader.Close()
			}()

			go func() {
				n, err := io.Copy(pipeWriter, dlResp.Body)
				if err != nil {
					goto ret
				}
				if n != fsize {
					err = ErrIncompleteDownload
				}
			ret:
				pipeWriter.CloseWithError(err)
				errChan <- err
			}()

			go func() {
				uparams := telbot.UploadParams{
					ChatId: app.Bot.Self.Id,
					Method: "sendDocument",
				}
				msg, err := app.Bot.UploadFile(jobCtx, uparams, telbot.FileReader{
					Reader:   pipeReader,
					Kind:     "document",
					FileName: fname,
				})
				if err != nil {
					pipeReader.CloseWithError(err)
				}
				res.msg = msg
				errChan <- err
			}()

			for range 2 {
				select {
				case err = <-errChan:
					if err != nil {
						return err
					}
				case <-jobCtx.Done():
					return jobCtx.Err()
				}
			}

			return nil
		}()

		if res.error == nil && res.msg != nil && res.msg.Document != nil {
			// TODO: add uploaded file info to database
		}

		job.resChan <- res
	}
}

// func (app *App) worker(ctx context.Context) {
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		default:
// 		}
// 		job := <-app.jobChan
// 		func() {
// 			res := dlResult{}
// 			defer func() { job.resChan <- res }()
// 			dlFile, err := app.downloadAndSaveFile(ctx, job.url)
// 			if err != nil {
// 				res.error = err
// 				return
// 			}
// 			defer os.Remove(dlFile.name)
// 			app.Log.Println("file downloaded:", dlFile.name)
// 			app.Log.Println("uploading file:", dlFile.name)
// 			msg, err := app.UploadFile(dlFile.name, dlFile.name)
// 			if err != nil {
// 				res.error = err
// 				return
// 			}
// 			res.msg = msg
// 		}()
// 	}
// }

func getFileName(resp *http.Response) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fname, ok := params["filename"]; ok && fname != "" {
				return fname
			}
		}
	}
	if resp.Request != nil && resp.Request.URL != nil {
		fname := path.Base(resp.Request.URL.Path)
		if fname != "." && fname != "/" {
			return fname
		}
	}
	return ""
}

func getRemoteFileInfo(ctx context.Context, fileUrl string) (fname string, fsize int64, err error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", fileUrl, nil)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	fsize, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return
	}

	fname = getFileName(resp)
	return
}

// func (app *App) downloadAndSaveFile(ctx context.Context, fileUrl string) (*dlFile, error) {
// 	fname, fsize, err := getRemoteFileInfo(ctx, fileUrl)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if fname == "" {
// 		return nil, ErrEmptyFileName
// 	}
// 	if fsize > maxFileSize {
// 		return nil, ErrMaxFileSize
// 	}
// 	req, err := http.NewRequestWithContext(ctx, "GET", fileUrl, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	resp, err := http.DefaultClient.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()
// 	app.Log.Printf("downloading file (%dKB): %s\n", fsize/1024, fname)
// 	f, err := os.Create(fname)
// 	if err != nil {
// 		return nil, err
// 	}
// 	ioCtx, ioCancel := context.WithTimeout(ctx, time.Minute*60)
// 	defer ioCancel()
// 	done := make(chan error)
// 	go func() {
// 		var n int64
// 		var err error
// 		n, err = io.Copy(f, resp.Body)
// 		if n != fsize {
// 			err = errors.New("file does not downloaded properly")
// 		}
// 		done <- err
// 	}()
// 	select {
// 	case err := <-done:
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &dlFile{name: fname, size: fsize}, nil
// 	case <-ctx.Done():
// 		return nil, ioCtx.Err()
// 	}
// }

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
