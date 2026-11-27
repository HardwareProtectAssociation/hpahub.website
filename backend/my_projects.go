package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const projectEditInterval = 24 * time.Hour

var errProjectEditCooldown = errors.New("project information can only be changed once every 24 hours")

func validProjectRiskLevel(value string) bool {
	return value == "" || value == "low" || value == "medium" || value == "high" || value == "extreme"
}

func validProjectTaskDifficulty(value string) bool {
	return value == "" || value == "easy" || value == "normal" || value == "hard" || value == "extreme"
}

func projectEditAvailability(project *core.Record, now time.Time) (bool, time.Time) {
	lastEdit := project.GetInt("project_last_edit_at")
	if lastEdit <= 0 {
		return true, time.Time{}
	}
	nextEdit := time.Unix(int64(lastEdit), 0).Add(projectEditInterval)
	return !now.Before(nextEdit), nextEdit
}

type myProject struct {
	OwnerRedacted  bool   `json:"ownerRedacted,omitempty"`
	ID             string `json:"id"`
	ProjectNumber  string `json:"projectNumber"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Status         string `json:"status"`
	ReviewStatus   string `json:"reviewStatus"`
	Difficulty     string `json:"difficulty"`
	OwnerName      string `json:"ownerName"`
	OwnerAvatarURL string `json:"ownerAvatarUrl"`
	Updated        string `json:"updated"`
	Version        int    `json:"version"`
}

func projectSnapshotMap(value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var snapshot map[string]any
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return nil, err
	}
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	return snapshot, nil
}

func projectSnapshotChanges(before, after map[string]any) []map[string]any {
	keys := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	changes := make([]map[string]any, 0, len(ordered))
	appendChange := func(field, detailKey string, oldValue any, hadOld bool, newValue any, hasNew bool) {
		if hadOld == hasNew && reflect.DeepEqual(oldValue, newValue) {
			return
		}
		var oldAny, newAny any
		if hadOld {
			oldAny = oldValue
		}
		if hasNew {
			newAny = newValue
		}
		change := map[string]any{"field": field, "before": oldAny, "after": newAny}
		if detailKey != "" {
			change["detailKey"] = detailKey
		}
		changes = append(changes, change)
	}
	for _, key := range ordered {
		if key == "description" {
			continue
		}
		oldValue, hadOld := before[key]
		newValue, hasNew := after[key]
		oldMap, oldIsMap := oldValue.(map[string]any)
		newMap, newIsMap := newValue.(map[string]any)
		if (oldIsMap || !hadOld) && (newIsMap || !hasNew) && (oldIsMap || newIsMap) {
			mapKeys := map[string]bool{}
			for detailKey := range oldMap {
				mapKeys[detailKey] = true
			}
			for detailKey := range newMap {
				mapKeys[detailKey] = true
			}
			detailKeys := make([]string, 0, len(mapKeys))
			for detailKey := range mapKeys {
				detailKeys = append(detailKeys, detailKey)
			}
			sort.Strings(detailKeys)
			for _, detailKey := range detailKeys {
				oldDetail, hadOldDetail := oldMap[detailKey]
				newDetail, hasNewDetail := newMap[detailKey]
				appendChange(key, detailKey, oldDetail, hadOldDetail, newDetail, hasNewDetail)
			}
			continue
		}
		oldArray, oldIsArray := oldValue.([]any)
		newArray, newIsArray := newValue.([]any)
		if (oldIsArray || !hadOld) && (newIsArray || !hasNew) && (oldIsArray || newIsArray) {
			oldItems, newItems := map[string]any{}, map[string]any{}
			for _, item := range oldArray {
				if encoded, err := json.Marshal(item); err == nil {
					oldItems[string(encoded)] = item
				}
			}
			for _, item := range newArray {
				if encoded, err := json.Marshal(item); err == nil {
					newItems[string(encoded)] = item
				}
			}
			itemKeys := map[string]bool{}
			for itemKey := range oldItems {
				itemKeys[itemKey] = true
			}
			for itemKey := range newItems {
				itemKeys[itemKey] = true
			}
			orderedItems := make([]string, 0, len(itemKeys))
			for itemKey := range itemKeys {
				orderedItems = append(orderedItems, itemKey)
			}
			sort.Strings(orderedItems)
			for _, itemKey := range orderedItems {
				oldItem, hadOldItem := oldItems[itemKey]
				newItem, hasNewItem := newItems[itemKey]
				appendChange(key, "", oldItem, hadOldItem, newItem, hasNewItem)
			}
			continue
		}
		appendChange(key, "", oldValue, hadOld, newValue, hasNew)
	}
	return changes
}

var validProjectNeeds = map[string]bool{"funding": true, "technical": true, "equipment": true, "venue": true, "manpower": true, "promotion": true, "emotional": true}
var validProjectOffers = map[string]bool{"technical": true, "funding": true, "equipment": true, "venue": true, "manpower": true, "promotion": true, "title": true}

func cleanProjectSelection(values []string, allowed map[string]bool) ([]string, bool) {
	cleaned := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if !allowed[value] {
			return nil, false
		}
		if !seen[value] {
			seen[value] = true
			cleaned = append(cleaned, value)
		}
	}
	return cleaned, true
}

func projectSelectionDetails(project *core.Record, field string) map[string]string {
	result := map[string]string{}
	_ = project.UnmarshalJSONField(field, &result)
	return result
}

func cleanProjectDetails(details map[string]string, selected []string) (map[string]string, bool) {
	cleaned := map[string]string{}
	allowed := map[string]bool{}
	for _, value := range selected {
		allowed[value] = true
	}
	for key, value := range details {
		if !allowed[key] {
			continue
		}
		value = strings.TrimSpace(value)
		if key == "title" && (value == "" || utf8.RuneCountInString(value) > 10) {
			return nil, false
		}
		if utf8.RuneCountInString(value) > 500 {
			return nil, false
		}
		if value != "" {
			cleaned[key] = value
		}
	}
	if allowed["title"] && cleaned["title"] == "" {
		return nil, false
	}
	return cleaned, true
}

func bindMyProjectRoutes(e *core.ServeEvent) {
	e.Router.POST("/api/hpahub/me/projects", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		creator, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to create project.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		var input struct {
			Title                    string            `json:"title"`
			Sponsor                  string            `json:"sponsor"`
			Summary                  string            `json:"summary"`
			Description              string            `json:"description"`
			CollaborationMode        string            `json:"collaborationMode"`
			JoinMode                 string            `json:"joinMode"`
			AcceptsSponsorship       bool              `json:"acceptsSponsorship"`
			AllowOthersManagement    bool              `json:"allowOthersManagement"`
			ConfidentialityRequested bool              `json:"confidentialityRequested"`
			ConfidentialityTerms     string            `json:"confidentialityTerms"`
			EstimatedDurationDays    int               `json:"estimatedDurationDays"`
			LongTerm                 bool              `json:"longTerm"`
			Completed                bool              `json:"completed"`
			ExecutionStatus          string            `json:"executionStatus"`
			PlannedStartDate         string            `json:"plannedStartDate"`
			ScheduleNote             string            `json:"scheduleNote"`
			ProposedDifficulty       string            `json:"proposedDifficulty"`
			DangerLevel              string            `json:"dangerLevel"`
			TaskDifficulty           string            `json:"taskDifficulty"`
			AdventureStyle           string            `json:"adventureStyle"`
			ProjectNature            string            `json:"projectNature"`
			ProjectNeedDetails       map[string]string `json:"projectNeedDetails"`
			ProviderOfferDetails     map[string]string `json:"providerOfferDetails"`
			ProjectNeeds             []string          `json:"projectNeeds"`
			ProviderOffers           []string          `json:"providerOffers"`
			StatusReason             string            `json:"statusReason"`
			Repeatable               bool              `json:"repeatable"`
			RepeatLimit              int               `json:"repeatLimit"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid project.", err)
		}
		input.Title = strings.TrimSpace(input.Title)
		input.Sponsor = strings.TrimSpace(input.Sponsor)
		input.Summary = strings.TrimSpace(input.Summary)
		input.Description = strings.TrimSpace(input.Description)
		input.ScheduleNote = strings.TrimSpace(input.ScheduleNote)
		input.ConfidentialityTerms = strings.TrimSpace(input.ConfidentialityTerms)
		if (input.ConfidentialityRequested && input.ConfidentialityTerms == "") || utf8.RuneCountInString(input.ConfidentialityTerms) > 10000 {
			return e.BadRequestError("Invalid confidentiality agreement.", nil)
		}
		if utf8.RuneCountInString(input.Description) > 10000 || utf8.RuneCountInString(input.ScheduleNote) > 2000 || utf8.RuneCountInString(input.Sponsor) > 500 || input.EstimatedDurationDays < 0 || input.EstimatedDurationDays > 36500 {
			return e.BadRequestError("Invalid project details.", nil)
		}
		if input.ExecutionStatus == "" {
			if input.Completed {
				input.ExecutionStatus = "completed"
			} else {
				input.ExecutionStatus = "ongoing"
			}
		}
		if input.ExecutionStatus != "not_started" && input.ExecutionStatus != "ongoing" && input.ExecutionStatus != "long_term" && input.ExecutionStatus != "paused" && input.ExecutionStatus != "completed" && input.ExecutionStatus != "terminated" {
			return e.BadRequestError("Invalid project status.", nil)
		}
		if input.ExecutionStatus == "long_term" {
			input.LongTerm = true
		}
		if input.Repeatable {
			if input.ExecutionStatus == "completed" || input.RepeatLimit < 1 || input.RepeatLimit > 9999 {
				return e.BadRequestError("Repeatable projects need an active status and a completion limit from 1 to 9999.", nil)
			}
		} else {
			input.RepeatLimit = 1
		}
		input.StatusReason = strings.TrimSpace(input.StatusReason)
		if input.ExecutionStatus == "paused" || input.ExecutionStatus == "terminated" {
			if input.StatusReason == "" || utf8.RuneCountInString(input.StatusReason) > 2000 {
				return e.BadRequestError("Invalid project status reason.", nil)
			}
		} else {
			input.StatusReason = ""
		}
		if input.LongTerm && input.EstimatedDurationDays != 0 {
			return e.BadRequestError("Long-term projects cannot have an estimated duration.", nil)
		}
		if input.PlannedStartDate != "" {
			if _, err := time.Parse("2006-01-02", input.PlannedStartDate); err != nil {
				return e.BadRequestError("Invalid start date.", nil)
			}
		}
		if input.CollaborationMode == "" {
			input.CollaborationMode = "collaborative"
		}
		if input.JoinMode == "" {
			input.JoinMode = "open"
		}
		if (input.CollaborationMode != "independent" && input.CollaborationMode != "collaborative") || (input.JoinMode != "open" && input.JoinMode != "approval") {
			return e.BadRequestError("Invalid collaboration settings.", nil)
		}
		if input.Title == "" || utf8.RuneCountInString(input.Title) > 160 || utf8.RuneCountInString(input.Summary) > 2000 {
			return e.BadRequestError("Invalid project.", nil)
		}
		if input.ProposedDifficulty != "" && !validProjectDifficulty(input.ProposedDifficulty) {
			return e.BadRequestError("Invalid difficulty.", nil)
		}
		if !validProjectRiskLevel(input.DangerLevel) || !validProjectTaskDifficulty(input.TaskDifficulty) {
			return e.BadRequestError("Invalid project risk or task difficulty.", nil)
		}
		if input.ProjectNature != "" && input.ProjectNature != "commercial" && input.ProjectNature != "learning" && input.ProjectNature != "showcase" && input.ProjectNature != "research" {
			return e.BadRequestError("Invalid project purpose.", nil)
		}
		projectNeeds, ok := cleanProjectSelection(input.ProjectNeeds, validProjectNeeds)
		if !ok {
			return e.BadRequestError("Invalid project needs.", nil)
		}
		providerOffers, ok := cleanProjectSelection(input.ProviderOffers, validProjectOffers)
		if !ok {
			return e.BadRequestError("Invalid provider offers.", nil)
		}
		needDetails, ok := cleanProjectDetails(input.ProjectNeedDetails, projectNeeds)
		if !ok {
			return e.BadRequestError("Invalid project need details.", nil)
		}
		offerDetails, ok := cleanProjectDetails(input.ProviderOfferDetails, providerOffers)
		if !ok {
			return e.BadRequestError("Invalid provider offer details.", nil)
		}
		if input.AdventureStyle == "" {
			input.AdventureStyle = "project"
		}
		if input.AdventureStyle != "project" && input.AdventureStyle != "dungeon" && input.AdventureStyle != "boss" {
			return e.BadRequestError("Invalid adventure style.", nil)
		}
		collection, err := e.App.FindCollectionByNameOrId("projects")
		if err != nil {
			return e.InternalServerError("Unable to create project.", err)
		}
		project := core.NewRecord(collection)
		project.Set("creator_user_id", creator.Id)
		project.Set("owner_user_id", creator.Id)
		project.Set("title", input.Title)
		project.Set("sponsor", input.Sponsor)
		project.Set("danger_level", input.DangerLevel)
		project.Set("task_difficulty", input.TaskDifficulty)
		project.Set("summary", input.Summary)
		project.Set("review_status", "approved")
		project.Set("execution_status", input.ExecutionStatus)
		project.Set("is_repeatable", input.Repeatable)
		project.Set("repeat_limit", input.RepeatLimit)
		project.Set("status_reason", input.StatusReason)
		project.Set("is_long_term", input.LongTerm)
		project.Set("collaboration_mode", input.CollaborationMode)
		project.Set("join_mode", input.JoinMode)
		project.Set("accepts_sponsorship", input.AcceptsSponsorship)
		project.Set("allow_others_management", input.AllowOthersManagement)
		project.Set("confidentiality_requested", input.ConfidentialityRequested)
		if input.ConfidentialityRequested {
			project.Set("confidentiality_terms", input.ConfidentialityTerms)
		}
		project.Set("description", map[string]string{"text": input.Description})
		project.Set("schedule_note", input.ScheduleNote)
		if input.PlannedStartDate != "" {
			project.Set("planned_start_date", input.PlannedStartDate)
		}
		if input.EstimatedDurationDays > 0 {
			project.Set("estimated_duration_days", input.EstimatedDurationDays)
		}
		project.Set("adventure_style", input.AdventureStyle)
		project.Set("project_nature", input.ProjectNature)
		project.Set("project_needs", projectNeeds)
		project.Set("provider_offers", providerOffers)
		project.Set("project_need_details", needDetails)
		project.Set("provider_offer_details", offerDetails)
		project.Set("version", 1)
		project.Set("published_at", time.Now())
		if input.ProposedDifficulty != "" {
			project.Set("owner_proposed_difficulty", input.ProposedDifficulty)
		}
		projectNumber := ""
		err = e.App.RunInTransaction(func(tx core.App) error {
			var err error
			projectNumber, err = uniqueProjectNumber(tx)
			if err != nil {
				return err
			}
			project.Set("project_number", projectNumber)
			if err := tx.Save(project); err != nil {
				return err
			}
			versions, err := tx.FindCollectionByNameOrId("project_versions")
			if err != nil {
				return err
			}
			snapshot := core.NewRecord(versions)
			snapshot.Set("project_id", project.Id)
			snapshot.Set("version", 1)
			snapshot.Set("editor_id", creator.Id)
			snapshot.Set("snapshot", map[string]any{
				"title":                    input.Title,
				"sponsor":                  input.Sponsor,
				"summary":                  input.Summary,
				"description":              input.Description,
				"collaborationMode":        input.CollaborationMode,
				"joinMode":                 input.JoinMode,
				"acceptsSponsorship":       input.AcceptsSponsorship,
				"allowOthersManagement":    input.AllowOthersManagement,
				"confidentialityRequested": input.ConfidentialityRequested,
				"confidentialityTerms":     input.ConfidentialityTerms,
				"estimatedDurationDays":    input.EstimatedDurationDays,
				"longTerm":                 input.LongTerm,
				"executionStatus":          input.ExecutionStatus,
				"repeatable":               input.Repeatable,
				"repeatLimit":              input.RepeatLimit,
				"plannedStartDate":         input.PlannedStartDate,
				"scheduleNote":             input.ScheduleNote,
				"proposedDifficulty":       input.ProposedDifficulty,
				"dangerLevel":              input.DangerLevel,
				"taskDifficulty":           input.TaskDifficulty,
				"adventureStyle":           input.AdventureStyle,
				"projectNature":            input.ProjectNature,
				"projectNeeds":             projectNeeds,
				"providerOffers":           providerOffers,
				"projectNeedDetails":       needDetails,
				"providerOfferDetails":     offerDetails,
				"statusReason":             input.StatusReason,
			})
			return tx.Save(snapshot)
		})
		if err != nil {
			return e.InternalServerError("Unable to create project.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "id": project.Id, "projectNumber": projectNumber})
	})
	e.Router.POST("/api/hpahub/me/projects/{id}/cover", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to save cover.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || (project.GetString("owner_user_id") != member.Id && project.GetString("creator_user_id") != member.Id) {
			return e.NotFoundError("Not found.", nil)
		}
		if err := e.Request.ParseMultipartForm(4 << 20); err != nil {
			return e.BadRequestError("Invalid cover.", err)
		}
		defer e.Request.MultipartForm.RemoveAll()
		files := e.Request.MultipartForm.File["cover"]
		if len(files) != 1 {
			return e.BadRequestError("Invalid cover.", nil)
		}
		file, err := filesystem.NewFileFromMultipart(files[0])
		if err != nil {
			return e.BadRequestError("Invalid cover.", err)
		}
		project.Set("cover", file)
		if err := e.App.Save(project); err != nil {
			return e.BadRequestError("Unable to save cover.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "coverUrl": projectCoverURL(project)})
	})
	loadOwned := func(e *core.RequestEvent) (*core.Record, *core.Record, error) {
		if e.Auth == nil {
			return nil, nil, e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return nil, nil, e.InternalServerError("Unable to load project.", err)
		}
		if !ok {
			return nil, nil, e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("deleted_at") != "" || (project.GetString("owner_user_id") != member.Id && project.GetString("creator_user_id") != member.Id) {
			return nil, nil, e.NotFoundError("Not found.", nil)
		}
		return project, member, nil
	}
	e.Router.DELETE("/api/hpahub/me/projects/{id}/cover", func(e *core.RequestEvent) error {
		noCache(e)
		project, _, err := loadOwned(e)
		if err != nil {
			return err
		}
		project.Set("cover", "")
		if err := e.App.Save(project); err != nil {
			return e.InternalServerError("Unable to remove cover.", err)
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
	e.Router.GET("/api/hpahub/me/projects/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		project, _, err := loadOwned(e)
		if err != nil {
			return err
		}
		canEdit, nextEdit := projectEditAvailability(project, time.Now().UTC())
		nextEditAt := ""
		if !nextEdit.IsZero() {
			nextEditAt = nextEdit.UTC().Format(time.RFC3339)
		}
		return e.JSON(200, map[string]any{
			"id":                   project.Id,
			"projectNumber":        project.GetString("project_number"),
			"title":                project.GetString("title"),
			"sponsor":              project.GetString("sponsor"),
			"dangerLevel":          project.GetString("danger_level"),
			"taskDifficulty":       project.GetString("task_difficulty"),
			"summary":              project.GetString("summary"),
			"description":          projectDescriptionText(project),
			"projectNature":        project.GetString("project_nature"),
			"executionStatus":      project.GetString("execution_status"),
			"repeatable":           project.GetBool("is_repeatable"),
			"repeatLimit":          max(project.GetInt("repeat_limit"), 1),
			"statusReason":         project.GetString("status_reason"),
			"projectNeeds":         project.GetStringSlice("project_needs"),
			"providerOffers":       project.GetStringSlice("provider_offers"),
			"projectNeedDetails":   projectSelectionDetails(project, "project_need_details"),
			"providerOfferDetails": projectSelectionDetails(project, "provider_offer_details"),
			"version":              project.GetInt("version"),
			"coverUrl":             projectCoverURL(project),
			"canEdit":              canEdit,
			"nextEditAt":           nextEditAt,
		})
	})
	e.Router.GET("/api/hpahub/me/projects/{id}/versions", func(e *core.RequestEvent) error {
		noCache(e)
		project, _, err := loadOwned(e)
		if err != nil {
			return err
		}
		rows, err := e.App.FindRecordsByFilter("project_versions", "project_id = {:project}", "-version", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load versions.", err)
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			editorName := ""
			if editor, err := e.App.FindRecordById("users", row.GetString("editor_id")); err == nil {
				editorName = publicPersonName(editor)
			}
			items = append(items, map[string]any{
				"version":    row.GetInt("version"),
				"created":    row.GetString("created"),
				"editorName": editorName,
			})
		}
		return e.JSON(200, map[string]any{"items": items})
	})
	e.Router.GET("/api/hpahub/me/projects/{id}/versions/{version}", func(e *core.RequestEvent) error {
		noCache(e)
		project, _, err := loadOwned(e)
		if err != nil {
			return err
		}
		version, err := strconv.Atoi(e.Request.PathValue("version"))
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		row, err := e.App.FindFirstRecordByFilter("project_versions", "project_id = {:project} && version = {:version}", dbx.Params{"project": project.Id, "version": version})
		if err != nil {
			return e.NotFoundError("Not found.", nil)
		}
		editorName := ""
		if editor, err := e.App.FindRecordById("users", row.GetString("editor_id")); err == nil {
			editorName = publicPersonName(editor)
		}
		after, err := projectSnapshotMap(row.Get("snapshot"))
		if err != nil {
			return e.InternalServerError("Unable to read project version.", err)
		}
		before := map[string]any{}
		if version > 1 {
			previous, previousErr := e.App.FindFirstRecordByFilter("project_versions", "project_id = {:project} && version = {:version}", dbx.Params{"project": project.Id, "version": version - 1})
			if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
				return e.InternalServerError("Unable to compare project versions.", previousErr)
			}
			if previousErr == nil {
				before, err = projectSnapshotMap(previous.Get("snapshot"))
				if err != nil {
					return e.InternalServerError("Unable to read previous project version.", err)
				}
			}
		}
		return e.JSON(200, map[string]any{
			"version":    row.GetInt("version"),
			"created":    row.GetString("created"),
			"editorName": editorName,
			"isInitial":  version == 1,
			"changes":    projectSnapshotChanges(before, after),
		})
	})
	e.Router.PATCH("/api/hpahub/me/projects/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		project, member, err := loadOwned(e)
		if err != nil {
			return err
		}
		var input struct {
			Title                string            `json:"title"`
			Sponsor              string            `json:"sponsor"`
			DangerLevel          string            `json:"dangerLevel"`
			TaskDifficulty       string            `json:"taskDifficulty"`
			Summary              string            `json:"summary"`
			Description          string            `json:"description"`
			ProjectNature        string            `json:"projectNature"`
			ExecutionStatus      string            `json:"executionStatus"`
			ProjectNeedDetails   map[string]string `json:"projectNeedDetails"`
			ProviderOfferDetails map[string]string `json:"providerOfferDetails"`
			ProjectNeeds         []string          `json:"projectNeeds"`
			ProviderOffers       []string          `json:"providerOffers"`
			StatusReason         string            `json:"statusReason"`
			Repeatable           bool              `json:"repeatable"`
			RepeatLimit          int               `json:"repeatLimit"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid project.", err)
		}
		input.Title = strings.TrimSpace(input.Title)
		input.Sponsor = strings.TrimSpace(input.Sponsor)
		if !validProjectRiskLevel(input.DangerLevel) || !validProjectTaskDifficulty(input.TaskDifficulty) {
			return e.BadRequestError("Invalid project risk or task difficulty.", nil)
		}
		input.Summary = strings.TrimSpace(input.Summary)
		input.Description = strings.TrimSpace(input.Description)
		if input.Title == "" || utf8.RuneCountInString(input.Title) > 160 || utf8.RuneCountInString(input.Sponsor) > 500 || utf8.RuneCountInString(input.Summary) > 2000 || utf8.RuneCountInString(input.Description) > 10000 {
			return e.BadRequestError("Invalid project details.", nil)
		}
		if input.ProjectNature != "" && input.ProjectNature != "commercial" && input.ProjectNature != "learning" && input.ProjectNature != "showcase" && input.ProjectNature != "research" {
			return e.BadRequestError("Invalid project purpose.", nil)
		}
		if input.ExecutionStatus != "not_started" && input.ExecutionStatus != "ongoing" && input.ExecutionStatus != "long_term" && input.ExecutionStatus != "paused" && input.ExecutionStatus != "completed" && input.ExecutionStatus != "terminated" {
			return e.BadRequestError("Invalid project status.", nil)
		}
		if input.Repeatable {
			if input.ExecutionStatus == "completed" || input.RepeatLimit < 1 || input.RepeatLimit > 9999 {
				return e.BadRequestError("Repeatable projects need an active status and a completion limit from 1 to 9999.", nil)
			}
		} else {
			input.RepeatLimit = 1
		}
		input.StatusReason = strings.TrimSpace(input.StatusReason)
		if input.ExecutionStatus == "paused" || input.ExecutionStatus == "terminated" {
			if input.StatusReason == "" || utf8.RuneCountInString(input.StatusReason) > 2000 {
				return e.BadRequestError("Invalid project status reason.", nil)
			}
		} else {
			input.StatusReason = ""
		}
		needs, ok := cleanProjectSelection(input.ProjectNeeds, validProjectNeeds)
		if !ok {
			return e.BadRequestError("Invalid project needs.", nil)
		}
		offers, ok := cleanProjectSelection(input.ProviderOffers, validProjectOffers)
		if !ok {
			return e.BadRequestError("Invalid provider offers.", nil)
		}
		if input.ProjectNeedDetails == nil {
			input.ProjectNeedDetails = projectSelectionDetails(project, "project_need_details")
		}
		if input.ProviderOfferDetails == nil {
			input.ProviderOfferDetails = projectSelectionDetails(project, "provider_offer_details")
		}
		needDetails, ok := cleanProjectDetails(input.ProjectNeedDetails, needs)
		if !ok {
			return e.BadRequestError("Invalid project need details.", nil)
		}
		offerDetails, ok := cleanProjectDetails(input.ProviderOfferDetails, offers)
		if !ok {
			return e.BadRequestError("Invalid provider offer details.", nil)
		}
		nextVersion := project.GetInt("version") + 1
		if nextVersion < 2 {
			nextVersion = 2
		}
		now := time.Now().UTC()
		err = e.App.RunInTransaction(func(tx core.App) error {
			result, err := tx.DB().NewQuery(`UPDATE projects SET project_last_edit_at = {:now}
				WHERE id = {:id} AND (COALESCE(project_last_edit_at, 0) = 0 OR project_last_edit_at <= {:cutoff})`).Bind(dbx.Params{
				"now": now.Unix(), "id": project.Id, "cutoff": now.Add(-projectEditInterval).Unix(),
			}).Execute()
			if err != nil {
				return err
			}
			if affected, err := result.RowsAffected(); err != nil {
				return err
			} else if affected != 1 {
				return errProjectEditCooldown
			}
			project.Set("project_last_edit_at", now.Unix())
			project.Set("title", input.Title)
			project.Set("sponsor", input.Sponsor)
			project.Set("danger_level", input.DangerLevel)
			project.Set("task_difficulty", input.TaskDifficulty)
			project.Set("summary", input.Summary)
			project.Set("description", map[string]string{"text": input.Description})
			project.Set("project_nature", input.ProjectNature)
			project.Set("execution_status", input.ExecutionStatus)
			project.Set("is_repeatable", input.Repeatable)
			project.Set("repeat_limit", input.RepeatLimit)
			project.Set("status_reason", input.StatusReason)
			project.Set("project_needs", needs)
			project.Set("provider_offers", offers)
			project.Set("project_need_details", needDetails)
			project.Set("provider_offer_details", offerDetails)
			project.Set("version", nextVersion)
			if err := tx.Save(project); err != nil {
				return err
			}
			versions, err := tx.FindCollectionByNameOrId("project_versions")
			if err != nil {
				return err
			}
			snapshot := core.NewRecord(versions)
			snapshot.Set("project_id", project.Id)
			snapshot.Set("version", nextVersion)
			snapshot.Set("editor_id", member.Id)
			snapshot.Set("snapshot", map[string]any{
				"title":                input.Title,
				"sponsor":              input.Sponsor,
				"dangerLevel":          input.DangerLevel,
				"taskDifficulty":       input.TaskDifficulty,
				"summary":              input.Summary,
				"description":          input.Description,
				"projectNature":        input.ProjectNature,
				"executionStatus":      input.ExecutionStatus,
				"repeatable":           input.Repeatable,
				"repeatLimit":          input.RepeatLimit,
				"projectNeeds":         needs,
				"providerOffers":       offers,
				"projectNeedDetails":   needDetails,
				"providerOfferDetails": offerDetails,
				"statusReason":         input.StatusReason,
			})
			return tx.Save(snapshot)
		})
		if errors.Is(err, errProjectEditCooldown) {
			latest, findErr := e.App.FindRecordById("projects", project.Id)
			if findErr != nil {
				return e.InternalServerError("Unable to check project edit limit.", findErr)
			}
			_, nextEdit := projectEditAvailability(latest, time.Now().UTC())
			return e.JSON(429, map[string]any{"message": errProjectEditCooldown.Error(), "nextProjectEditAt": nextEdit.UTC().Format(time.RFC3339)})
		}
		if err != nil {
			return e.BadRequestError("Unable to save project.", err)
		}
		return e.JSON(200, map[string]any{"ok": true, "version": nextVersion, "nextProjectEditAt": now.Add(projectEditInterval).Format(time.RFC3339)})
	})
	e.Router.GET("/api/hpahub/me/projects", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		member, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		initiated := []myProject{}
		participated := []myProject{}
		followed := []myProject{}
		seen := map[string]bool{}
		converted := func(project *core.Record) myProject {
			ownerName := ""
			ownerAvatarURL := ""
			ownerRedacted := false
			if owner, err := e.App.FindRecordById("users", project.GetString("owner_user_id")); err == nil {
				ownerName = publicPersonName(owner)
				ownerRedacted = profileSectionHidden(owner, "projects") || profileSectionHiddenForViewer(e.App, owner, "project:"+project.Id, e.Auth)
				if ownerRedacted {
					ownerName = maskedText(ownerName)
				}
				if !ownerRedacted && owner.GetString("avatar") != "" {
					ownerAvatarURL = contentURL("members", owner.Id, "avatar")
				}
			}
			return myProject{
				OwnerRedacted:  ownerRedacted,
				ID:             project.Id,
				ProjectNumber:  project.GetString("project_number"),
				Title:          project.GetString("title"),
				Summary:        project.GetString("summary"),
				Status:         project.GetString("execution_status"),
				ReviewStatus:   project.GetString("review_status"),
				Difficulty:     project.GetString("difficulty"),
				OwnerName:      ownerName,
				OwnerAvatarURL: ownerAvatarURL,
				Updated:        project.GetString("updated"),
				Version:        project.GetInt("version"),
			}
		}
		owned, err := e.App.FindRecordsByFilter("projects", `owner_user_id = {:user} && deleted_at = ""`, "-updated", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		for _, project := range owned {
			if seen[project.Id] {
				continue
			}
			seen[project.Id] = true
			initiated = append(initiated, converted(project))
		}
		memberships, err := e.App.FindRecordsByFilter("project_members", `user_id = {:user} && status = "active"`, "", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		for _, membership := range memberships {
			project, err := e.App.FindRecordById("projects", membership.GetString("project_id"))
			if err != nil || project.GetString("owner_user_id") == member.Id || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
				continue
			}
			if seen[project.Id] {
				continue
			}
			seen[project.Id] = true
			participated = append(participated, converted(project))
		}
		follows, err := e.App.FindRecordsByFilter("project_follows", `user_id = {:user} && is_active = true`, "", 0, 0, dbx.Params{"user": member.Id})
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		for _, follow := range follows {
			project, err := e.App.FindRecordById("projects", follow.GetString("project_id"))
			if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
				continue
			}
			if seen[project.Id] {
				continue
			}
			seen[project.Id] = true
			followed = append(followed, converted(project))
		}
		return e.JSON(200, map[string]any{
			"initiated":    initiated,
			"participated": participated,
			"followed":     followed,
		})
	})

	e.Router.POST("/api/hpahub/projects/{id}/follow", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, ok, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to follow project.", err)
		}
		if !ok {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("owner_user_id") == viewer.Id {
			return e.BadRequestError("You cannot follow your own project.", nil)
		}
		follow, err := e.App.FindFirstRecordByFilter("project_follows", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": viewer.Id})
		if errors.Is(err, sql.ErrNoRows) {
			collection, err := e.App.FindCollectionByNameOrId("project_follows")
			if err != nil {
				return e.InternalServerError("Unable to follow project.", err)
			}
			follow = core.NewRecord(collection)
			follow.Set("project_id", project.Id)
			follow.Set("user_id", viewer.Id)
			follow.Set("is_active", true)
			follow.Set("receive_updates", true)
			follow.Set("followed_at", time.Now())
		} else if err != nil {
			return e.InternalServerError("Unable to follow project.", err)
		} else {
			follow.Set("is_active", !follow.GetBool("is_active"))
			if follow.GetBool("is_active") {
				follow.Set("followed_at", time.Now())
			}
		}
		if err := e.App.Save(follow); err != nil {
			return e.InternalServerError("Unable to follow project.", err)
		}
		return e.JSON(200, map[string]any{"following": follow.GetBool("is_active")})
	})
}
