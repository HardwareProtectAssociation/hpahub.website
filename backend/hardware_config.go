package main

import (
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type hardwareConfigItem struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Detail   string `json:"detail"`
}

func bindHardwareConfigRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/me/hardware-config", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load hardware config.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		rows, err := e.App.FindRecordsByFilter("user_hardware_configs",
			"user_id = {:user}", "sort_order", 0, 0, dbx.Params{"user": owner.Id})
		if err != nil {
			return e.InternalServerError("Unable to load hardware config.", err)
		}
		items := make([]hardwareConfigItem, 0, len(rows))
		for _, row := range rows {
			items = append(items, hardwareConfigItem{
				ID:       row.Id,
				Category: row.GetString("category"),
				Name:     row.GetString("name"),
				Detail:   row.GetString("detail"),
			})
		}
		return e.JSON(200, map[string]any{"items": items})
	})

	e.Router.PUT("/api/hpahub/me/hardware-config", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to save hardware config.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input struct {
			Items []hardwareConfigItem `json:"items"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid hardware config.", err)
		}
		if len(input.Items) > 50 {
			return e.BadRequestError("Too many items.", nil)
		}
		for _, item := range input.Items {
			item.Name = strings.TrimSpace(item.Name)
			item.Category = strings.TrimSpace(item.Category)
			if item.Name == "" || item.Category == "" || utf8.RuneCountInString(item.Name) > 200 || utf8.RuneCountInString(item.Category) > 50 || utf8.RuneCountInString(item.Detail) > 500 {
				return e.BadRequestError("Invalid hardware config.", nil)
			}
		}
		err = e.App.RunInTransaction(func(tx core.App) error {
			// Delete all existing items
			existing, err := tx.FindRecordsByFilter("user_hardware_configs",
				"user_id = {:user}", "", 0, 0, dbx.Params{"user": owner.Id})
			if err != nil {
				return err
			}
			for _, row := range existing {
				if err := tx.Delete(row); err != nil {
					return err
				}
			}
			// Create new items
			collection, err := tx.FindCollectionByNameOrId("user_hardware_configs")
			if err != nil {
				return err
			}
			for i, item := range input.Items {
				record := core.NewRecord(collection)
				record.Set("user_id", owner.Id)
				record.Set("category", item.Category)
				record.Set("name", item.Name)
				record.Set("detail", item.Detail)
				record.Set("sort_order", i)
				if err := tx.Save(record); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return e.InternalServerError("Unable to save hardware config.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "count": len(input.Items)})
	})
}
