package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/thehxdev/bahador/db"
	"github.com/thehxdev/bahador/utils"
	"github.com/thehxdev/telbot"
	conv "github.com/thehxdev/telbot/ext/conversation"
)

const (
	maxFileSize  int64  = 4 * 1024 * 1024 * 1024
	filePartSize int64  = 200 * 1024 * 1024
	workersCount int    = 5
	tokenEnvVar  string = "BAHADOR_BOT_TOKEN"
	hostEnvVar   string = "BAHADOR_BOT_HOST"
	dbPathEnvVar string = "BAHADOR_DB_PATH"
)

type jobResult struct {
	error
	fileIds []string
}

type dlJob struct {
	url         string
	resChan     chan jobResult
	cancelChan  chan struct{}
	eventLogger func(string, ...any)
}

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
	if _, err := exec.LookPath("7zz"); err != nil {
		return nil, fmt.Errorf("7zz command not found: %v", err)
	}

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
		logEvent := job.eventLogger

		res := func() jobResult {
			// app.Log.Println("processing job:", job.url)

			jobCtx, jobCancel := context.WithCancel(ctx)
			defer jobCancel()

			go func() {
				<-job.cancelChan
				jobCancel()
			}()

			app.Log.Println("Getting remote file information")
			fname, fsize, err := getRemoteFileInfo(jobCtx, job.url)
			if err != nil {
				return jobResult{error: err}
			}

			if fsize > maxFileSize {
				return jobResult{error: ErrMaxFileSize}
			}

			var result jobResult
			if fsize <= filePartSize {
				app.Log.Println("Processing job with pipe")
				result = app.processJobWithPipe(jobCtx, fname, fsize, job.url, logEvent)
			} else {
				app.Log.Println("Processing job with download")
				result = app.processJobWithDownload(jobCtx, fname, fsize, job.url, logEvent)
			}

			return result
		}()

		job.resChan <- res
	}
}

func (app *App) processJobWithPipe(ctx context.Context, fname string, fsize int64, url string, logEvent func(string, ...any)) (res jobResult) {
	if fname == "" {
		res.error = ErrEmptyFileName
		return
	}

	res.error = func() error {
		pCtx, pCancel := context.WithTimeout(ctx, time.Minute*30)
		defer pCancel()

		dlReq, err := http.NewRequestWithContext(pCtx, "GET", url, nil)
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

		logEvent("Processing download and upload with pipe")

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
			files := []telbot.IFileInfo{
				&telbot.FileReader{
					Reader:   pipeReader,
					Kind:     "document",
					FileName: fname,
				},
			}
			msg, err := app.Bot.UploadFile(pCtx, uparams, files)
			if err != nil {
				pipeReader.CloseWithError(err)
			} else {
				res.fileIds = []string{msg.Document.FileId}
			}
			errChan <- err
		}()

		for range 2 {
			select {
			case err = <-errChan:
				if err != nil {
					return err
				}
			case <-pCtx.Done():
				return pCtx.Err()
			}
		}

		return nil
	}()

	return
}

func (app *App) processJobWithDownload(ctx context.Context, fname string, fsize int64, url string, logEvent func(string, ...any)) (res jobResult) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "bahador_*")
	if err != nil {
		res.error = err
		return
	}
	defer os.RemoveAll(tmpDir)

	app.Log.Println("tmp dir:", tmpDir)

	pCtx, pCancel := context.WithTimeout(ctx, time.Minute*90)
	defer pCancel()

	fileDlPath := filepath.Join(tmpDir, fname)
	app.Log.Println("File download path:", fileDlPath)
	logEvent("Downloading the file...")

	err = app.downloadAndSaveFile(pCtx, fileDlPath, fsize, url)
	if err != nil {
		res.error = err
		return
	}

	archivePath := filepath.Join(tmpDir, fname+".7z")
	app.Log.Println("Archive path:", archivePath)
	logEvent("Creating archive files...")
	parts, err := SplitFileToParts(ctx, fileDlPath, archivePath, "200m")
	if err != nil {
		res.error = err
		return
	}
	// app.Log.Printf("parts: %#v\n", parts)

	partsCount := len(parts)
	fileIdChan := make(chan string, partsCount)

	logEvent("Uploading %d parts...", partsCount)
	for _, p := range parts {
		go func(pPath string) {
			var fileId string = ""
			defer func() { fileIdChan <- fileId }()
			f, err := os.Open(pPath)
			if err != nil {
				return
			}
			defer f.Close()
			uparams := telbot.UploadParams{
				ChatId: app.Bot.Self.Id,
				Method: "sendDocument",
			}
			app.Log.Println("Uploading file:", pPath)
			files := []telbot.IFileInfo{
				&telbot.FileReader{
					Reader:   f,
					FileName: filepath.Base(pPath),
					Kind:     "document",
				},
			}
			msg, err := app.Bot.UploadFile(pCtx, uparams, files)
			if err != nil {
				return
			}
			fileId = msg.Document.FileId
		}(p)
	}

	fileIds := []string{}
	for range partsCount {
		select {
		case id := <-fileIdChan:
			if id == "" {
				// FIXME: this error must be `ErrIncompleteUpload`
				res.error = ErrIncompleteDownload
				return
			}
			fileIds = append(fileIds, id)
		case <-pCtx.Done():
			res.error = pCtx.Err()
			return
		}
	}

	res.fileIds = fileIds
	return
}

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

func (app *App) downloadAndSaveFile(ctx context.Context, fpath string, fsize int64, fileUrl string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fileUrl, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(fpath)
	if err != nil {
		return err
	}
	n, err := utils.CopyWithContext(ctx, f, resp.Body)
	if err != nil {
		return err
	}
	if n != fsize {
		return ErrIncompleteDownload
	}
	return nil
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

func (app *App) ConvAuthMiddleware(next conv.ConversationHandler) conv.ConversationHandler {
	return func(c *conv.Conversation, update telbot.Update) error {
		if _, err := app.DB.UserAuthenticate(update.Message.From.Id); err == nil {
			return next(c, update)
		}
		return nil
	}
}
