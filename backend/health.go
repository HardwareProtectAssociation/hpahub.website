package main

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type healthCheckin struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Mood      string `json:"mood"`
	Rested    bool   `json:"rested"`
	Exercised bool   `json:"exercised"`
	AteOnTime bool   `json:"ateOnTime"`
	Note      string `json:"note"`
	Created   string `json:"created"`
}

func bindHealthRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/me/health", func(e *core.RequestEvent) error {
		noCache(e)
		owner, member, err := healthOwner(e)
		if err != nil {
			return err
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		date := time.Now().Format("2006-01-02")
		checkin, err := todayHealthCheckin(e.App, owner.Id, date)
		if err != nil {
			return e.InternalServerError("Unable to load health check.", err)
		}
		history, err := e.App.FindRecordsByFilter("health_checkins", "user_id = {:user}", "-check_date", 7, 0, dbx.Params{"user": owner.Id})
		if err != nil {
			return e.InternalServerError("Unable to load health check.", err)
		}
		items := make([]healthCheckin, 0, len(history))
		for _, record := range history {
			items = append(items, healthRecord(record))
		}
		return e.JSON(200, map[string]any{
			"userId":         owner.Id,
			"enabled":        owner.GetBool("daily_health_enabled"),
			"date":           date,
			"checkedInToday": checkin != nil,
			"today":          checkin,
			"history":        items,
		})
	})

	e.Router.POST("/api/hpahub/me/health/checkin", func(e *core.RequestEvent) error {
		noCache(e)
		owner, member, err := healthOwner(e)
		if err != nil {
			return err
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input struct {
			Mood      string `json:"mood"`
			Rested    bool   `json:"rested"`
			Exercised bool   `json:"exercised"`
			AteOnTime bool   `json:"ateOnTime"`
			Note      string `json:"note"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid health check.", err)
		}
		input.Mood = strings.TrimSpace(input.Mood)
		if !validHealthMood(input.Mood) || utf8.RuneCountInString(input.Note) > 300 {
			return e.BadRequestError("Invalid health check.", nil)
		}
		date := time.Now().Format("2006-01-02")
		record, err := todayHealthCheckin(e.App, owner.Id, date)
		if err != nil {
			return e.InternalServerError("Unable to save health check.", err)
		}
		if record == nil {
			collection, err := e.App.FindCollectionByNameOrId("health_checkins")
			if err != nil {
				return e.InternalServerError("Unable to save health check.", err)
			}
			record = core.NewRecord(collection)
			record.Set("user_id", owner.Id)
			record.Set("check_date", date)
		}
		record.Set("mood", input.Mood)
		record.Set("rested", input.Rested)
		record.Set("exercised", input.Exercised)
		record.Set("ate_on_time", input.AteOnTime)
		record.Set("note", strings.TrimSpace(input.Note))
		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("Unable to save health check.", err)
		}
		return e.JSON(200, healthRecord(record))
	})

	e.Router.PATCH("/api/hpahub/me/health/settings", func(e *core.RequestEvent) error {
		noCache(e)
		owner, member, err := healthOwner(e)
		if err != nil {
			return err
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input struct {
			Enabled bool `json:"enabled"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid health settings.", err)
		}
		owner.Set("daily_health_enabled", input.Enabled)
		if err := e.App.Save(owner); err != nil {
			return e.InternalServerError("Unable to save health settings.", err)
		}
		return e.JSON(200, map[string]any{"enabled": input.Enabled})
	})
}

func healthOwner(e *core.RequestEvent) (*core.Record, bool, error) {
	if e.Auth == nil {
		return nil, false, e.UnauthorizedError("Authentication required.", nil)
	}
	owner, member, err := personalCenterOwner(e.App, e.Auth)
	if err != nil {
		return nil, false, e.InternalServerError("Unable to load health check.", err)
	}
	return owner, member, nil
}

func todayHealthCheckin(app core.App, userID, date string) (*core.Record, error) {
	record, err := app.FindFirstRecordByFilter("health_checkins", "user_id = {:user} && check_date = {:date}", dbx.Params{"user": userID, "date": date})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

func healthRecord(record *core.Record) healthCheckin {
	return healthCheckin{
		ID:        record.Id,
		Date:      record.GetString("check_date"),
		Mood:      record.GetString("mood"),
		Rested:    record.GetBool("rested"),
		Exercised: record.GetBool("exercised"),
		AteOnTime: record.GetBool("ate_on_time"),
		Note:      record.GetString("note"),
		Created:   record.GetString("created"),
	}
}

func validHealthMood(value string) bool {
	for _, option := range []string{"great", "good", "okay", "low", "bad"} {
		if value == option {
			return true
		}
	}
	return false
}
