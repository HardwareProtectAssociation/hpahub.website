package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

type hardwareField struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Group     string   `json:"group"`
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	DataType  string   `json:"dataType"`
	Unit      string   `json:"unit"`
	Required  bool     `json:"required"`
	Options   []string `json:"options"`
	HelpText  string   `json:"helpText"`
	SortOrder float64  `json:"sortOrder"`
}
type hardwareTable struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Key       string           `json:"key"`
	Title     string           `json:"title"`
	Columns   []map[string]any `json:"columns"`
	SortOrder float64          `json:"sortOrder"`
}

func hardwareAdmin(e *core.RequestEvent) error {
	if e.Auth == nil {
		return e.UnauthorizedError("Authentication required.", nil)
	}
	if siteRole(e.Auth) != "admin" && siteRole(e.Auth) != "octopus" {
		return e.ForbiddenError("Insufficient permissions.", nil)
	}
	return nil
}
func hardwareMember(e *core.RequestEvent) (*core.Record, error) {
	if e.Auth == nil {
		return nil, e.UnauthorizedError("Authentication required.", nil)
	}
	owner, member, err := personalCenterOwner(e.App, e.Auth)
	if err != nil {
		return nil, e.InternalServerError("Unable to load member profile.", err)
	}
	if !member || (siteRole(owner) != "member" && siteRole(owner) != "admin") {
		return nil, e.ForbiddenError("A member profile is required.", nil)
	}
	return owner, nil
}
func hardwareCanManage(e *core.RequestEvent, record *core.Record) bool {
	if e.Auth == nil {
		return false
	}
	if siteRole(e.Auth) == "admin" || siteRole(e.Auth) == "octopus" {
		return true
	}
	owner, member, err := personalCenterOwner(e.App, e.Auth)
	return err == nil && member && record.GetString("submitted_by") == owner.Id
}
func hardwareJSON(e *core.RequestEvent, target any) error {
	if err := json.NewDecoder(http.MaxBytesReader(e.Response, e.Request.Body, 2<<20)).Decode(target); err != nil {
		return e.BadRequestError("Invalid hardware data.", err)
	}
	return nil
}
func hardwareTypeData(e *core.RequestEvent, public bool) (map[string]any, error) {
	types, err := e.App.FindRecordsByFilter("hw_types", "", "sort_order,label", 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(types))
	for _, typ := range types {
		fields, err := e.App.FindRecordsByFilter("hw_fields", "type = {:id}", "sort_order,id", 0, 0, dbx.Params{"id": typ.Id})
		if err != nil {
			return nil, err
		}
		tables, err := e.App.FindRecordsByFilter("hw_tables", "type = {:id}", "sort_order,id", 0, 0, dbx.Params{"id": typ.Id})
		if err != nil {
			return nil, err
		}
		fs := make([]hardwareField, 0, len(fields))
		for _, r := range fields {
			var opts []string
			_ = r.UnmarshalJSONField("options", &opts)
			fs = append(fs, hardwareField{r.Id, r.GetString("type"), r.GetString("group"), r.GetString("key"), r.GetString("label"), r.GetString("data_type"), r.GetString("unit"), r.GetBool("required"), opts, r.GetString("help_text"), r.GetFloat("sort_order")})
		}
		ts := make([]hardwareTable, 0, len(tables))
		for _, r := range tables {
			var cols []map[string]any
			_ = r.UnmarshalJSONField("columns", &cols)
			ts = append(ts, hardwareTable{r.Id, r.GetString("type"), r.GetString("key"), r.GetString("title"), cols, r.GetFloat("sort_order")})
		}
		out = append(out, map[string]any{"id": typ.Id, "key": typ.GetString("key"), "label": typ.GetString("label"), "fields": fs, "tables": ts})
	}
	return map[string]any{"items": out}, nil
}
func bindHardwareDatabaseRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hardware/schema", func(e *core.RequestEvent) error {
		noCache(e)
		v, err := hardwareTypeData(e, false)
		if err != nil {
			return e.InternalServerError("Unable to load hardware schema.", err)
		}
		return e.JSON(200, v)
	})
	e.Router.POST("/api/hardware/types", func(e *core.RequestEvent) error {
		noCache(e)
		if err := hardwareAdmin(e); err != nil {
			return err
		}
		var in struct {
			Key   string `json:"key"`
			Label string `json:"label"`
		}
		if err := hardwareJSON(e, &in); err != nil {
			return err
		}
		in.Key = strings.TrimSpace(in.Key)
		in.Label = strings.TrimSpace(in.Label)
		if in.Key == "" || len(in.Key) > 48 || in.Label == "" || len([]rune(in.Label)) > 80 {
			return e.BadRequestError("Invalid hardware type.", nil)
		}
		c, _ := e.App.FindCollectionByNameOrId("hw_types")
		r := core.NewRecord(c)
		r.Set("key", in.Key)
		r.Set("label", in.Label)
		r.Set("name_field", "name")
		if err := e.App.Save(r); err != nil {
			return e.BadRequestError("Hardware type already exists.", err)
		}
		return e.JSON(201, map[string]string{"id": r.Id})
	})
	e.Router.POST("/api/hardware/fields", func(e *core.RequestEvent) error {
		noCache(e)
		if err := hardwareAdmin(e); err != nil {
			return err
		}
		var in hardwareField
		if err := hardwareJSON(e, &in); err != nil {
			return err
		}
		if in.Type == "" || strings.TrimSpace(in.Key) == "" || strings.TrimSpace(in.Label) == "" {
			return e.BadRequestError("Invalid hardware field.", nil)
		}
		if in.DataType == "" {
			in.DataType = "text"
		}
		valid := map[string]bool{"text": true, "number": true, "boolean": true, "date": true, "select": true, "multiselect": true}
		if !valid[in.DataType] {
			return e.BadRequestError("Invalid field type.", nil)
		}
		c, _ := e.App.FindCollectionByNameOrId("hw_fields")
		r := core.NewRecord(c)
		r.Set("type", in.Type)
		r.Set("group", strings.TrimSpace(in.Group))
		if in.Group == "" {
			r.Set("group", "基本信息")
		}
		r.Set("key", strings.TrimSpace(in.Key))
		r.Set("label", strings.TrimSpace(in.Label))
		r.Set("data_type", in.DataType)
		r.Set("unit", strings.TrimSpace(in.Unit))
		r.Set("required", in.Required)
		r.Set("options", in.Options)
		r.Set("help_text", in.HelpText)
		r.Set("sort_order", in.SortOrder)
		if err := e.App.Save(r); err != nil {
			return e.BadRequestError("Unable to save field.", err)
		}
		return e.JSON(201, map[string]string{"id": r.Id})
	})
	e.Router.POST("/api/hardware/tables", func(e *core.RequestEvent) error {
		noCache(e)
		if err := hardwareAdmin(e); err != nil {
			return err
		}
		var in hardwareTable
		if err := hardwareJSON(e, &in); err != nil {
			return err
		}
		if in.Type == "" || in.Key == "" || in.Title == "" || len(in.Columns) == 0 || len(in.Columns) > 40 {
			return e.BadRequestError("Invalid test table.", nil)
		}
		c, _ := e.App.FindCollectionByNameOrId("hw_tables")
		r := core.NewRecord(c)
		r.Set("type", in.Type)
		r.Set("key", in.Key)
		r.Set("title", in.Title)
		r.Set("columns", in.Columns)
		r.Set("sort_order", in.SortOrder)
		if err := e.App.Save(r); err != nil {
			return e.BadRequestError("Unable to save test table.", err)
		}
		return e.JSON(201, map[string]string{"id": r.Id})
	})
	e.Router.GET("/api/hardware/items", func(e *core.RequestEvent) error {
		noCache(e)
		filter := `is_public = true`
		params := dbx.Params{}
		if e.Auth != nil {
			if viewer, member, err := personalCenterOwner(e.App, e.Auth); err == nil && member {
				filter = `(is_public = true || submitted_by = {:viewer})`
				params["viewer"] = viewer.Id
			}
		}
		if (siteRole(e.Auth) == "admin" || siteRole(e.Auth) == "octopus") && e.Request.URL.Query().Get("all") == "true" {
			filter = ""
		}
		if kind := e.Request.URL.Query().Get("type"); kind != "" {
			filter += " && kind = {:kind}"
			params["kind"] = kind
		}
		page, _ := strconv.Atoi(e.Request.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		const limit = 30
		rows, err := e.App.FindRecordsByFilter("hardware", filter, "sort_order,name", limit+1, (page-1)*limit, params)
		if err != nil {
			return e.InternalServerError("Unable to load hardware.", err)
		}
		hasMore := len(rows) > limit
		if hasMore {
			rows = rows[:limit]
		}
		items := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			items = append(items, hardwareRecordData(r))
		}
		return e.JSON(200, map[string]any{"items": items, "hasMore": hasMore})
	})
	e.Router.GET("/api/hardware/items/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		r, err := e.App.FindRecordById("hardware", e.Request.PathValue("id"))
		if err != nil || (!r.GetBool("is_public") && !hardwareCanManage(e, r)) {
			return e.NotFoundError("Not found.", nil)
		}
		return e.JSON(200, hardwareRecordData(r))
	})
	e.Router.POST("/api/hardware/items", func(e *core.RequestEvent) error {
		noCache(e)
		owner, err := hardwareMember(e)
		if err != nil {
			return err
		}
		var in map[string]any
		if err := hardwareJSON(e, &in); err != nil {
			return err
		}
		return saveHardwareRecord(e, nil, owner, in)
	})
	e.Router.PATCH("/api/hardware/items/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		r, err := e.App.FindRecordById("hardware", e.Request.PathValue("id"))
		if err != nil || !hardwareCanManage(e, r) {
			return e.NotFoundError("Not found.", nil)
		}
		owner, _, ownerErr := personalCenterOwner(e.App, e.Auth)
		if ownerErr != nil {
			return e.InternalServerError("Unable to load member profile.", ownerErr)
		}
		if owner == nil {
			owner = e.Auth
		}
		var in map[string]any
		if err := hardwareJSON(e, &in); err != nil {
			return err
		}
		return saveHardwareRecord(e, r, owner, in)
	})
	e.Router.GET("/api/hardware/items/{id}/articles", func(e *core.RequestEvent) error {
		noCache(e)
		id := e.Request.PathValue("id")
		rows, err := e.App.FindRecordsByFilter("hardware_articles", "hardware_id = {:id} && published = true", "-updated", 100, 0, dbx.Params{"id": id})
		if err != nil {
			return e.InternalServerError("Unable to load articles.", err)
		}
		out := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			out = append(out, hardwareArticleData(r))
		}
		return e.JSON(200, map[string]any{"items": out})
	})
	e.Router.GET("/api/hardware/articles/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		r, err := e.App.FindRecordById("hardware_articles", e.Request.PathValue("id"))
		if err != nil || !r.GetBool("published") {
			return e.NotFoundError("Not found.", nil)
		}
		return e.JSON(200, hardwareArticleData(r))
	})
	e.Router.GET("/api/hardware/articles/{id}/images/{file}", func(e *core.RequestEvent) error {
		noCache(e)
		r, err := e.App.FindRecordById("hardware_articles", e.Request.PathValue("id"))
		if err != nil || !r.GetBool("published") {
			return e.NotFoundError("Not found.", nil)
		}
		return serveContentFile(e, r, "images", e.Request.PathValue("file"))
	})
	e.Router.POST("/api/hardware/articles", func(e *core.RequestEvent) error {
		noCache(e)
		owner, err := hardwareMember(e)
		if err != nil {
			return err
		}
		if err := e.Request.ParseMultipartForm(32 << 20); err != nil {
			return e.BadRequestError("Invalid article.", err)
		}
		defer e.Request.MultipartForm.RemoveAll()
		var id, title, summary, body string
		published := true
		id = e.Request.FormValue("hardwareId")
		title = strings.TrimSpace(e.Request.FormValue("title"))
		summary = e.Request.FormValue("summary")
		body = e.Request.FormValue("body")
		if title == "" || len([]rune(title)) > 160 || len(body) > 1<<20 {
			return e.BadRequestError("Invalid article.", nil)
		}
		hw, err := e.App.FindRecordById("hardware", id)
		if err != nil || (!hw.GetBool("is_public") && !hardwareCanManage(e, hw)) {
			return e.NotFoundError("Not found.", nil)
		}
		c, _ := e.App.FindCollectionByNameOrId("hardware_articles")
		r := core.NewRecord(c)
		r.Set("hardware_id", hw.Id)
		r.Set("author_user_id", owner.Id)
		r.Set("title", title)
		r.Set("summary", summary)
		var blocks any
		if json.Unmarshal([]byte(body), &blocks) != nil {
			return e.BadRequestError("Invalid article body.", nil)
		}
		r.Set("body", blocks)
		r.Set("published", published)
		uploads := e.Request.MultipartForm.File["images"]
		if len(uploads) > 20 {
			return e.BadRequestError("Too many images.", nil)
		}
		for _, header := range uploads {
			f, err := filesystem.NewFileFromMultipart(header)
			if err != nil {
				return e.BadRequestError("Invalid image.", err)
			}
			r.Set("images+", f)
		}
		if err := e.App.Save(r); err != nil {
			return e.BadRequestError("Unable to save article.", err)
		}
		if published {
			hw.Set("is_public", true)
			if err := e.App.Save(hw); err != nil {
				return e.InternalServerError("Unable to publish hardware.", err)
			}
		}
		return e.JSON(201, hardwareArticleData(r))
	})
}
func hardwareRecordData(r *core.Record) map[string]any {
	var data map[string]any
	_ = r.UnmarshalJSONField("data", &data)
	return map[string]any{"id": r.Id, "kind": r.GetString("kind"), "name": r.GetString("name"), "brand": r.GetString("brand"), "subtitle": r.GetString("subtitle"), "data": data, "cover": r.GetString("cover"), "isPublic": r.GetBool("is_public"), "updated": r.GetString("updated"), "submittedBy": r.GetString("submitted_by")}
}
func hardwareArticleData(r *core.Record) map[string]any {
	var body any
	_ = r.UnmarshalJSONField("body", &body)
	return map[string]any{"id": r.Id, "hardwareId": r.GetString("hardware_id"), "title": r.GetString("title"), "summary": r.GetString("summary"), "body": body, "images": r.GetStringSlice("images"), "published": r.GetBool("published"), "updated": r.GetString("updated")}
}
func saveHardwareRecord(e *core.RequestEvent, r *core.Record, owner *core.Record, in map[string]any) error {
	if r == nil {
		c, err := e.App.FindCollectionByNameOrId("hardware")
		if err != nil {
			return e.InternalServerError("Hardware collection unavailable.", err)
		}
		r = core.NewRecord(c)
		r.Set("submitted_by", owner.Id)
	}
	for _, key := range []string{"kind", "name", "brand", "subtitle"} {
		if val, ok := in[key]; ok {
			r.Set(key, val)
		}
	}
	if v, ok := in["data"]; ok {
		raw, err := json.Marshal(v)
		if err != nil || len(raw) > 8<<20 {
			return e.BadRequestError("Invalid test report.", err)
		}
		var data map[string]any
		if json.Unmarshal(raw, &data) != nil {
			return e.BadRequestError("Invalid test report.", nil)
		}
		if err := validateHardwareData(e.App, r.GetString("kind"), data); err != nil {
			return e.BadRequestError("Invalid test report.", err)
		}
		r.Set("data", data)
	}
	if v, ok := in["isPublic"]; ok {
		r.Set("is_public", v)
	}
	if strings.TrimSpace(r.GetString("kind")) == "" || strings.TrimSpace(r.GetString("name")) == "" {
		return e.BadRequestError("Hardware category and name are required.", nil)
	}
	if err := e.App.Save(r); err != nil {
		return e.BadRequestError("Unable to save hardware.", err)
	}
	return e.JSON(200, hardwareRecordData(r))
}

func validateHardwareData(app core.App, kind string, data map[string]any) error {
	typ, err := app.FindFirstRecordByFilter("hw_types", "key = {:key}", dbx.Params{"key": kind})
	if err != nil {
		return fmt.Errorf("unknown hardware category")
	}
	fields, err := app.FindRecordsByFilter("hw_fields", "type = {:id}", "", 0, 0, dbx.Params{"id": typ.Id})
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, field := range fields {
		key := field.GetString("key")
		allowed[key] = true
		value, exists := data[key]
		if field.GetBool("required") && (!exists || value == nil || value == "") {
			return fmt.Errorf("required field missing: %s", field.GetString("label"))
		}
		if !exists {
			continue
		}
		switch field.GetString("data_type") {
		case "number":
			if _, ok := value.(float64); !ok {
				return fmt.Errorf("field must be numeric: %s", key)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("field must be boolean: %s", key)
			}
		case "multiselect":
			if _, ok := value.([]any); !ok {
				return fmt.Errorf("field must be a list: %s", key)
			}
		case "select":
			options := field.GetStringSlice("options")
			if len(options) > 0 {
				found := false
				for _, option := range options {
					if value == option {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("invalid option: %s", key)
				}
			}
		}
	}
	tables, err := app.FindRecordsByFilter("hw_tables", "type = {:id}", "", 0, 0, dbx.Params{"id": typ.Id})
	if err != nil {
		return err
	}
	for _, table := range tables {
		key := table.GetString("key")
		allowed[key] = true
		rows, exists := data[key]
		if !exists {
			continue
		}
		list, ok := rows.([]any)
		if !ok || len(list) > 500 {
			return fmt.Errorf("invalid test table: %s", key)
		}
		var columns []map[string]any
		_ = table.UnmarshalJSONField("columns", &columns)
		for _, row := range list {
			item, ok := row.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid test row")
			}
			for col := range item {
				found := false
				for _, definition := range columns {
					if definition["key"] == col {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("unknown test column: %s", col)
				}
			}
		}
	}
	for key := range data {
		if !allowed[key] && key != "model" && key != "category" && key != "note" {
			return fmt.Errorf("unknown test field: %s", key)
		}
	}
	return nil
}
