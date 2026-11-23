package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

const autoPrefix = "@hpahub_auto_"

type policy struct {
	IntervalHours int `json:"intervalHours" db:"interval_hours"`
	RetainCount   int `json:"retainCount" db:"retain_count"`
	LastRun       int64 `json:"-" db:"last_run"`
}

func (p policy) valid() bool {
	return p.IntervalHours >= 0 && p.IntervalHours <= 8760 && p.RetainCount >= 1 && p.RetainCount <= 100
}

func (p policy) due(now time.Time) bool {
	return p.IntervalHours > 0 && now.Unix() >= p.LastRun+int64(p.IntervalHours)*3600
}

func loadPolicy(app core.App) (policy, error) {
	var p policy
	err := app.DB().NewQuery("SELECT interval_hours, retain_count, last_run FROM hpahub_backup_policy WHERE id = 1").One(&p)
	return p, err
}

func main() {
	app := pocketbase.New()
	var policyMu, backupMu sync.Mutex
	wake := make(chan struct{}, 1)
	stop := make(chan struct{})
	var stopOnce sync.Once

	// PocketBase's active-backup marker is not an atomic lock. Serialize the actual operations.
	lockBackup := func(e *core.BackupEvent) error {
		backupMu.Lock()
		defer backupMu.Unlock()
		return e.Next()
	}
	app.OnBackupCreate().BindFunc(lockBackup)
	app.OnBackupRestore().BindFunc(lockBackup)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		_, err := app.DB().NewQuery(`CREATE TABLE IF NOT EXISTS hpahub_backup_policy (
			id INTEGER PRIMARY KEY CHECK (id = 1), interval_hours INTEGER NOT NULL,
			retain_count INTEGER NOT NULL, last_run INTEGER NOT NULL
		)`).Execute()
		if err != nil { return err }
		if _, err = app.DB().NewQuery("INSERT OR IGNORE INTO hpahub_backup_policy VALUES (1, 0, 5, 0)").Execute(); err != nil { return err }

		e.Router.GET("/api/backup-policy", func(e *core.RequestEvent) error {
			policyMu.Lock()
			defer policyMu.Unlock()
			p, err := loadPolicy(e.App)
			if err != nil { return e.InternalServerError("Failed to load backup policy.", err) }
			return e.JSON(http.StatusOK, p)
		}).Bind(apis.RequireSuperuserAuth())
		e.Router.PUT("/api/backup-policy", func(e *core.RequestEvent) error {
			var input struct {
				IntervalHours *int `json:"intervalHours"`
				RetainCount *int `json:"retainCount"`
			}
			if err := e.BindBody(&input); err != nil {
				return e.BadRequestError("Invalid backup policy.", err)
			}
			if input.IntervalHours == nil || input.RetainCount == nil {
				return e.BadRequestError("Both backup policy fields are required.", nil)
			}
			p := policy{IntervalHours: *input.IntervalHours, RetainCount: *input.RetainCount}
			if !p.valid() {
				return e.BadRequestError("Invalid backup policy.", nil)
			}
			policyMu.Lock()
			defer policyMu.Unlock()
			_, err := e.App.DB().NewQuery("UPDATE hpahub_backup_policy SET interval_hours = {:interval}, retain_count = {:retain}, last_run = {:last} WHERE id = 1").Bind(dbx.Params{"interval": p.IntervalHours, "retain": p.RetainCount, "last": time.Now().Unix()}).Execute()
			if err != nil { return e.InternalServerError("Failed to save backup policy.", err) }
			select { case wake <- struct{}{}: default: }
			return e.JSON(http.StatusOK, p)
		}).Bind(apis.RequireSuperuserAuth())
		if !e.Router.HasRoute(http.MethodGet, "/{path...}") {
			e.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), true))
		}
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				runScheduled(app, &policyMu)
				select { case <-stop: return; case <-wake: case <-ticker.C: }
			}
		}()
		return e.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		stopOnce.Do(func() { close(stop) })
		return e.Next()
	})
	if err := app.Start(); err != nil { log.Fatal(err) }
}

func runScheduled(app core.App, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
	p, err := loadPolicy(app)
	if err != nil { app.Logger().Error("load backup policy", "error", err); return }
	if !p.due(time.Now()) { return }
	name := fmt.Sprintf("%s%d.zip", autoPrefix, time.Now().UnixNano())
	if err := app.CreateBackup(context.Background(), name); err != nil {
		app.Logger().Error("create scheduled backup", "error", err)
		return
	}
	_, err = app.DB().NewQuery("UPDATE hpahub_backup_policy SET last_run = {:last} WHERE id = 1").Bind(dbx.Params{"last": time.Now().Unix()}).Execute()
	if err != nil { app.Logger().Error("save backup time", "error", err); return }
	fsys, err := app.NewBackupsFilesystem()
	if err != nil { app.Logger().Error("open backup storage", "error", err); return }
	defer fsys.Close()
	files, err := fsys.List(autoPrefix)
	if err != nil { app.Logger().Error("list scheduled backups", "error", err); return }
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	if len(files) <= p.RetainCount { return }
	for _, f := range files[p.RetainCount:] {
		if err := fsys.Delete(f.Key); err != nil { app.Logger().Error("delete old scheduled backup", "key", f.Key, "error", err) }
	}
}
