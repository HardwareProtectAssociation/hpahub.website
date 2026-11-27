package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

type publicProject struct {
	CompletionNumber int                   `json:"completionNumber,omitempty"`
	Redacted         bool                  `json:"redacted,omitempty"`
	OwnerRedacted    bool                  `json:"ownerRedacted,omitempty"`
	CreatorName      string                `json:"creatorName,omitempty"`
	CreatorRedacted  bool                  `json:"creatorRedacted,omitempty"`
	CreatorAvatarURL string                `json:"creatorAvatarUrl,omitempty"`
	Managers         []publicProjectMember `json:"managers,omitempty"`
	ParticipantCount int                   `json:"participantCount,omitempty"`
	FollowCount      int                   `json:"followCount"`
	ID               string                `json:"id"`
	ProjectNumber    string                `json:"projectNumber"`
	Title            string                `json:"title"`
	Sponsor          string                `json:"sponsor"`
	Summary          string                `json:"summary"`
	Status           string                `json:"status"`
	ProjectNature    string                `json:"projectNature,omitempty"`
	Difficulty       string                `json:"difficulty"`
	DangerLevel      string                `json:"dangerLevel"`
	TaskDifficulty   string                `json:"taskDifficulty"`
	AdventureStyle   string                `json:"adventureStyle"`
	OwnerID          string                `json:"ownerId"`
	OwnerName        string                `json:"ownerName"`
	OwnerAvatarURL   string                `json:"ownerAvatarUrl"`
	PublishedAt      string                `json:"publishedAt"`
	CoverURL         string                `json:"coverUrl,omitempty"`
	Updated          string                `json:"updated"`
}

type publicProjectStage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
	Current     bool   `json:"current"`
}

type publicProjectMember struct {
	Redacted  bool   `json:"redacted,omitempty"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Duties    string `json:"duties"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type publicProjectDetail struct {
	publicProject
	Version                  int                   `json:"version"`
	Repeatable               bool                  `json:"repeatable"`
	RepeatLimit              int                   `json:"repeatLimit"`
	ViewerCompletionCount    int                   `json:"viewerCompletionCount"`
	ViewerCompletionPending  bool                  `json:"viewerCompletionPending"`
	CanRequestCompletion     bool                  `json:"canRequestCompletion"`
	Description              string                `json:"description"`
	CollaborationMode        string                `json:"collaborationMode"`
	ScheduleNote             string                `json:"scheduleNote"`
	ProjectNature            string                `json:"projectNature"`
	StatusReason             string                `json:"statusReason,omitempty"`
	ProjectNeeds             []string              `json:"projectNeeds"`
	ProviderOffers           []string              `json:"providerOffers"`
	ProjectNeedDetails       map[string]string     `json:"projectNeedDetails"`
	ProviderOfferDetails     map[string]string     `json:"providerOfferDetails"`
	AcceptsSponsorship       bool                  `json:"acceptsSponsorship"`
	AllowOthersManagement    bool                  `json:"allowOthersManagement"`
	ConfidentialityRequested bool                  `json:"confidentialityRequested"`
	ConfidentialityTerms     string                `json:"confidentialityTerms"`
	PlannedStartDate         string                `json:"plannedStartDate"`
	EstimatedDurationDays    int                   `json:"estimatedDurationDays"`
	LongTerm                 bool                  `json:"longTerm"`
	CurrentStage             string                `json:"currentStage"`
	Stages                   []publicProjectStage  `json:"stages"`
	Members                  []publicProjectMember `json:"members"`
	CanApply                 bool                  `json:"canApply"`
	ApplicationPending       bool                  `json:"applicationPending"`
	AlreadyMember            bool                  `json:"alreadyMember"`
	CanRequestLeave          bool                  `json:"canRequestLeave"`
	DepartureRequest         *memberDeparture      `json:"departureRequest,omitempty"`
	RemovalRequest           *memberDeparture      `json:"removalRequest,omitempty"`
	CanRequestImprovement    bool                  `json:"canRequestImprovement"`
	ImprovementPending       bool                  `json:"improvementPending"`
	CanFollow                bool                  `json:"canFollow"`
	Following                bool                  `json:"following"`
	JoinMode                 string                `json:"joinMode"`
	ProposedDifficulty       string                `json:"proposedDifficulty"`
	OwnerConfirmed           bool                  `json:"ownerConfirmed"`
	ApproveCount             int                   `json:"approveCount"`
	OpposeCount              int                   `json:"opposeCount"`
	IsOwner                  bool                  `json:"isOwner"`
}

type managedProject struct {
	ID                 string `json:"id"`
	ProjectNumber      string `json:"projectNumber"`
	Title              string `json:"title"`
	Summary            string `json:"summary"`
	Status             string `json:"status"`
	Difficulty         string `json:"difficulty"`
	ReviewStatus       string `json:"reviewStatus"`
	OwnerName          string `json:"ownerName"`
	OwnerID            string `json:"ownerId"`
	Created            string `json:"created"`
	Updated            string `json:"updated"`
	ProposedDifficulty string `json:"proposedDifficulty"`
	OwnerConfirmed     bool   `json:"ownerConfirmed"`
	ApproveCount       int    `json:"approveCount"`
	OpposeCount        int    `json:"opposeCount"`
}

func publicProjectInfoSnapshot(project *core.Record) map[string]any {
	return map[string]any{
		"title": project.GetString("title"), "summary": project.GetString("summary"), "sponsor": project.GetString("sponsor"), "collaborationMode": project.GetString("collaboration_mode"), "joinMode": project.GetString("join_mode"),
		"acceptsSponsorship": project.GetBool("accepts_sponsorship"), "allowOthersManagement": project.GetBool("allow_others_management"),
		"confidentialityRequested": project.GetBool("confidentiality_requested"), "estimatedDurationDays": project.GetInt("estimated_duration_days"),
		"longTerm": project.GetBool("is_long_term"), "executionStatus": project.GetString("execution_status"), "plannedStartDate": project.GetString("planned_start_date"),
		"scheduleNote": project.GetString("schedule_note"), "proposedDifficulty": project.GetString("owner_proposed_difficulty"),
		"dangerLevel": project.GetString("danger_level"), "taskDifficulty": project.GetString("task_difficulty"),
		"projectNature": project.GetString("project_nature"), "projectNeeds": project.GetStringSlice("project_needs"), "providerOffers": project.GetStringSlice("provider_offers"),
		"projectNeedDetails": projectSelectionDetails(project, "project_need_details"), "providerOfferDetails": projectSelectionDetails(project, "provider_offer_details"), "statusReason": project.GetString("status_reason"),
		"repeatable": project.GetBool("is_repeatable"), "repeatLimit": max(project.GetInt("repeat_limit"), 1),
	}
}

// bindPublicProjectRoutes exposes only approved, non-deleted projects. Draft,
// pending and rejected projects stay private.
func bindPublicProjectRoutes(e *core.ServeEvent) {
	bindProjectManagementRoutes(e)
	e.Router.GET("/api/hpahub/projects/{id}/cover", func(e *core.RequestEvent) error {
		noCache(e)
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		return serveContentFile(e, project, "cover", project.GetString("cover"))
	})
	e.Router.GET("/api/hpahub/projects", func(e *core.RequestEvent) error {
		noCache(e)
		offset := 0
		if value := e.Request.URL.Query().Get("offset"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 || parsed%20 != 0 {
				return e.BadRequestError("Invalid project offset.", nil)
			}
			offset = parsed
		}
		rows, err := e.App.FindRecordsByFilter("projects", `review_status = "approved" && deleted_at = ""`, "-published_at,-updated,-id", 21, offset)
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		owners := map[string]*core.Record{}
		hasMore := len(rows) > 20
		if hasMore {
			rows = rows[:20]
		}
		followCounts, err := projectFollowCounts(e.App, rows)
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		items := make([]publicProject, 0, len(rows))
		for _, project := range rows {
			if project.GetString("deleted_at") != "" {
				continue
			}
			card, err := projectHallCard(e, project, owners)
			if err != nil {
				return e.InternalServerError("Unable to load project details.", err)
			}
			card.FollowCount = followCounts[project.Id]
			items = append(items, card)
		}
		return e.JSON(200, map[string]any{"items": items, "hasMore": hasMore, "nextOffset": offset + 20})
	})
	e.Router.GET("/api/hpahub/projects/{id}/versions", func(e *core.RequestEvent) error {
		noCache(e)
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		rows, err := e.App.FindRecordsByFilter("project_versions", "project_id = {:project}", "-version", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load project versions.", err)
		}
		currentVersion := project.GetInt("version")
		if currentVersion < 1 {
			currentVersion = 1
		}
		items := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if row.GetInt("version") > currentVersion {
				currentVersion = row.GetInt("version")
			}
			items = append(items, map[string]any{
				"version": row.GetInt("version"), "created": row.GetString("created"),
				"editorName": publicProjectVersionEditor(e, project, row),
			})
		}
		if len(items) == 0 {
			created := project.GetString("published_at")
			if created == "" {
				created = project.GetString("created")
			}
			items = append(items, map[string]any{"version": currentVersion, "created": created})
		}
		return e.JSON(200, map[string]any{"currentVersion": currentVersion, "items": items})
	})
	e.Router.GET("/api/hpahub/projects/{id}/versions/{version}", func(e *core.RequestEvent) error {
		noCache(e)
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		version, err := strconv.Atoi(e.Request.PathValue("version"))
		if err != nil || version < 1 {
			return e.NotFoundError("Not found.", nil)
		}
		rows, err := e.App.FindRecordsByFilter("project_versions", "project_id = {:project} && version <= {:version}", "version", 0, 0, dbx.Params{"project": project.Id, "version": version})
		if err != nil {
			return e.InternalServerError("Unable to load project version.", err)
		}
		snapshot := publicProjectInfoSnapshot(project)
		var selected *core.Record
		for _, row := range rows {
			values, err := projectSnapshotMap(row.Get("snapshot"))
			if err != nil {
				return e.InternalServerError("Unable to read project version.", err)
			}
			for key, value := range values {
				snapshot[key] = value
			}
			if row.GetInt("version") == version {
				selected = row
			}
		}
		if selected == nil {
			currentVersion := project.GetInt("version")
			if currentVersion < 1 {
				currentVersion = 1
			}
			if version != currentVersion || len(rows) != 0 {
				return e.NotFoundError("Not found.", nil)
			}
		}
		delete(snapshot, "description")
		delete(snapshot, "confidentialityTerms")
		created := project.GetString("created")
		editorName := ""
		if selected != nil {
			created = selected.GetString("created")
			editorName = publicProjectVersionEditor(e, project, selected)
		}
		return e.JSON(200, map[string]any{"version": version, "created": created, "editorName": editorName, "snapshot": snapshot})
	})
	e.Router.GET("/api/hpahub/projects/{id}", func(e *core.RequestEvent) error {
		noCache(e)
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		owners := map[string]*core.Record{}
		card, err := projectHallCard(e, project, owners)
		if err != nil {
			return e.InternalServerError("Unable to load project details.", err)
		}
		followCounts, err := projectFollowCounts(e.App, []*core.Record{project})
		if err != nil {
			return e.InternalServerError("Unable to load project follows.", err)
		}
		card.FollowCount = followCounts[project.Id]
		detail := publicProjectDetail{
			publicProject:            card,
			Version:                  max(project.GetInt("version"), 1),
			Repeatable:               project.GetBool("is_repeatable"),
			RepeatLimit:              max(project.GetInt("repeat_limit"), 1),
			Description:              projectDescriptionText(project),
			CollaborationMode:        project.GetString("collaboration_mode"),
			ScheduleNote:             project.GetString("schedule_note"),
			ProjectNature:            project.GetString("project_nature"),
			StatusReason:             project.GetString("status_reason"),
			ProjectNeeds:             project.GetStringSlice("project_needs"),
			ProviderOffers:           project.GetStringSlice("provider_offers"),
			ProjectNeedDetails:       projectSelectionDetails(project, "project_need_details"),
			ProviderOfferDetails:     projectSelectionDetails(project, "provider_offer_details"),
			AcceptsSponsorship:       project.GetBool("accepts_sponsorship"),
			AllowOthersManagement:    project.GetBool("allow_others_management"),
			ConfidentialityRequested: project.GetBool("confidentiality_requested"),
			ConfidentialityTerms:     project.GetString("confidentiality_terms"),
			PlannedStartDate:         project.GetString("planned_start_date"),
			EstimatedDurationDays:    project.GetInt("estimated_duration_days"),
			LongTerm:                 project.GetBool("is_long_term"),
			Stages:                   []publicProjectStage{},
			Members:                  []publicProjectMember{},
			ProposedDifficulty:       project.GetString("owner_proposed_difficulty"),
			OwnerConfirmed:           project.GetBool("owner_difficulty_confirmed"),
			JoinMode:                 project.GetString("join_mode"),
		}
		// Compute vote counts
		votes, _ := e.App.FindRecordsByFilter("project_difficulty_votes",
			"project_id = {:project}", "", 0, 0, dbx.Params{"project": project.Id})
		for _, v := range votes {
			if v.GetString("stance") == "approve" {
				detail.ApproveCount++
			} else {
				detail.OpposeCount++
			}
		}
		// Check if viewer is owner
		if e.Auth != nil {
			if owner, member, err := personalCenterOwner(e.App, e.Auth); err == nil && member {
				if project.GetString("owner_user_id") == owner.Id {
					detail.IsOwner = true
				}
				if err := promoteExpiredProjectDepartures(e.App, project.Id); err != nil {
					return e.InternalServerError("Unable to update expired departure requests.", err)
				}
			}
		}
		stages, err := e.App.FindRecordsByFilter("project_stages", "project_id = {:project}", "sort_order", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load project stages.", err)
		}
		currentStageID := project.GetString("current_stage_id")
		for _, stage := range stages {
			current := stage.Id == currentStageID
			if current {
				detail.CurrentStage = stage.GetString("name")
			}
			detail.Stages = append(detail.Stages, publicProjectStage{
				ID:          stage.Id,
				Name:        stage.GetString("name"),
				Description: stage.GetString("description"),
				SortOrder:   stage.GetInt("sort_order"),
				StartedAt:   stage.GetString("started_at"),
				CompletedAt: stage.GetString("completed_at"),
				Current:     current,
			})
		}
		members, err := e.App.FindRecordsByFilter("project_members", `project_id = {:project} && status = "active"`, "joined_at", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return e.InternalServerError("Unable to load project members.", err)
		}
		for _, membership := range members {
			member, err := e.App.FindRecordById("users", membership.GetString("user_id"))
			if err != nil {
				continue
			}
			if profileSectionHidden(member, "projects") || profileSectionHiddenForViewer(e.App, member, "project:"+project.Id, e.Auth) {
				detail.Members = append(detail.Members, publicProjectMember{Redacted: true, Name: maskedText(publicPersonName(member)), Duties: maskedText(membership.GetString("duties"))})
				continue
			}
			detail.Members = append(detail.Members, publicProjectMember{
				ID: member.Id, Name: publicPersonName(member), Duties: membership.GetString("duties"),
			})
		}
		if e.Auth != nil {
			if viewer, member, err := personalCenterOwner(e.App, e.Auth); err == nil && member && viewer.Id != project.GetString("owner_user_id") {
				detail.CanFollow = true
				if follow, err := e.App.FindFirstRecordByFilter("project_follows", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
					detail.Following = follow.GetBool("is_active")
				}
				activeMember := false
				if _, err := e.App.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
					activeMember = true
					detail.AlreadyMember = true
				}
				if activeMember {
					if project.GetBool("is_repeatable") {
						approved, countErr := e.App.FindRecordsByFilter("project_completion_requests", `project_id = {:project} && user_id = {:user} && status = "approved"`, "", 0, 0, dbx.Params{"project": project.Id, "user": viewer.Id})
						if countErr != nil {
							return e.InternalServerError("Unable to load completion records.", countErr)
						}
						detail.ViewerCompletionCount = len(approved)
						_, pendingErr := e.App.FindFirstRecordByFilter("project_completion_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id})
						detail.ViewerCompletionPending = pendingErr == nil
						detail.CanRequestCompletion = !detail.ViewerCompletionPending && len(approved) < detail.RepeatLimit && project.GetString("execution_status") != "completed" && project.GetString("execution_status") != "terminated"
					}
					if departure, err := openDepartureForMember(e.App, project.Id, viewer.Id); err == nil {
						item := departureData(e.App, departure)
						if item.Type == "leave" {
							detail.DepartureRequest = &item
						} else {
							detail.RemovalRequest = &item
						}
					} else if errors.Is(err, sql.ErrNoRows) {
						detail.CanRequestLeave = true
					}
				}
				hasPending := false
				if pending, err := e.App.FindFirstRecordByFilter("project_join_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
					hasPending = true
					if pending.GetString("source") == "improvement" {
						detail.ImprovementPending = true
					} else {
						detail.ApplicationPending = true
					}
				}
				if !activeMember && !hasPending {
					if project.GetString("execution_status") == "completed" {
						detail.CanRequestImprovement = true
					} else if project.GetString("collaboration_mode") != "independent" {
						detail.CanApply = true
					}
				}
			}
		}
		return e.JSON(200, detail)
	})
	e.Router.POST("/api/hpahub/projects/{id}/apply", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load project.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("owner_user_id") == viewer.Id {
			return e.BadRequestError("You already own this project.", nil)
		}
		if project.GetString("execution_status") == "completed" {
			return e.BadRequestError("Completed projects accept improvement requests instead.", nil)
		}
		if project.GetString("collaboration_mode") == "independent" {
			return e.BadRequestError("This is an independent project.", nil)
		}

		var input struct {
			Message       string `json:"message"`
			Profession    string `json:"profession"`
			Time          string `json:"time"`
			Ability       string `json:"ability"`
			AcceptedTerms string `json:"acceptedTerms"`
		}
		if err := e.BindBody(&input); err != nil || len([]rune(strings.TrimSpace(input.Message))) > 500 {
			return e.BadRequestError("Invalid application.", nil)
		}
		if len([]rune(strings.TrimSpace(input.Profession))) > 100 || len([]rune(strings.TrimSpace(input.Time))) > 100 || len([]rune(strings.TrimSpace(input.Ability))) > 200 {
			return e.BadRequestError("Invalid application.", nil)
		}
		err = e.App.RunInTransaction(func(tx core.App) error {
			if err := saveConfidentialityAgreement(tx, project, viewer, input.AcceptedTerms); err != nil {
				return e.BadRequestError("Confidentiality agreement must be accepted.", err)
			}
			if _, err := tx.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
				return e.BadRequestError("You are already a project member.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return e.InternalServerError("Unable to load membership.", err)
			}
			if _, err := tx.FindFirstRecordByFilter("project_join_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
				return e.BadRequestError("You already have a pending application.", nil)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return e.InternalServerError("Unable to load application.", err)
			}
			collection, err := tx.FindCollectionByNameOrId("project_join_requests")
			if err != nil {
				return e.InternalServerError("Unable to submit application.", err)
			}
			request := core.NewRecord(collection)
			request.Set("project_id", project.Id)
			request.Set("user_id", viewer.Id)
			request.Set("source", "application")
			request.Set("message", strings.TrimSpace(input.Message))
			request.Set("apply_profession", strings.TrimSpace(input.Profession))
			request.Set("apply_time", strings.TrimSpace(input.Time))
			request.Set("apply_ability", strings.TrimSpace(input.Ability))
			request.Set("status", "pending")
			if err := tx.Save(request); err != nil {
				return e.InternalServerError("Unable to submit application.", err)
			}
			// Free join mode: auto-accept
			if project.GetString("join_mode") == "open" {
				request.Set("status", "accepted")
				request.Set("handled_at", time.Now())
				if err := tx.Save(request); err != nil {
					return e.InternalServerError("Unable to submit application.", err)
				}
				// Add as project member
				membership, err := tx.FindFirstRecordByFilter("project_members", "project_id = {:project} && user_id = {:user}", dbx.Params{"project": project.Id, "user": viewer.Id})
				if errors.Is(err, sql.ErrNoRows) {
					mCollection, err := tx.FindCollectionByNameOrId("project_members")
					if err != nil {
						return e.InternalServerError("Unable to add member.", err)
					}
					membership = core.NewRecord(mCollection)
					membership.Set("project_id", project.Id)
					membership.Set("user_id", viewer.Id)
				} else if err != nil {
					return e.InternalServerError("Unable to add member.", err)
				}
				if duties := joinRequestDuties(request); duties != "" {
					membership.Set("duties", duties)
				}
				membership.Set("status", "active")
				membership.Set("joined_at", time.Now())
				if err := tx.Save(membership); err != nil {
					return e.InternalServerError("Unable to add member.", err)
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"ok": true, "autoAccepted": project.GetString("join_mode") == "open"})
	})
	e.Router.POST("/api/hpahub/projects/{id}/improvement", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		viewer, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil {
			return e.InternalServerError("Unable to load project.", err)
		}
		if !member {
			return e.ForbiddenError("No member profile is linked to this account.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("execution_status") != "completed" {
			return e.BadRequestError("Only completed projects accept improvement requests.", nil)
		}
		if project.GetString("owner_user_id") == viewer.Id {
			return e.BadRequestError("You already own this project.", nil)
		}
		if _, err := e.App.FindFirstRecordByFilter("project_members", `project_id = {:project} && user_id = {:user} && status = "active"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
			return e.BadRequestError("You are already a project member.", nil)
		}
		if _, err := e.App.FindFirstRecordByFilter("project_join_requests", `project_id = {:project} && user_id = {:user} && status = "pending"`, dbx.Params{"project": project.Id, "user": viewer.Id}); err == nil {
			return e.BadRequestError("You already have a pending request.", nil)
		}
		var input struct {
			Message       string `json:"message"`
			AcceptedTerms string `json:"acceptedTerms"`
		}
		if err := e.BindBody(&input); err != nil || len([]rune(strings.TrimSpace(input.Message))) > 500 {
			return e.BadRequestError("Invalid improvement request.", nil)
		}
		err = e.App.RunInTransaction(func(tx core.App) error {
			if err := saveConfidentialityAgreement(tx, project, viewer, input.AcceptedTerms); err != nil {
				return e.BadRequestError("Confidentiality agreement must be accepted.", err)
			}
			collection, err := tx.FindCollectionByNameOrId("project_join_requests")
			if err != nil {
				return e.InternalServerError("Unable to submit improvement request.", err)
			}
			request := core.NewRecord(collection)
			request.Set("project_id", project.Id)
			request.Set("user_id", viewer.Id)
			request.Set("source", "improvement")
			request.Set("message", strings.TrimSpace(input.Message))
			request.Set("status", "pending")
			if err := tx.Save(request); err != nil {
				return e.InternalServerError("Unable to submit improvement request.", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"ok": true})
	})
	e.Router.GET("/api/hpahub/manage/projects", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		role := siteRole(e.Auth)
		if role != "admin" && role != "octopus" {
			return e.ForbiddenError("Insufficient permissions.", nil)
		}
		rows, err := e.App.FindRecordsByFilter("projects", `deleted_at = ""`, "-updated", 500, 0)
		if err != nil {
			return e.InternalServerError("Unable to load projects.", err)
		}
		owners := map[string]*core.Record{}
		items := make([]managedProject, 0, len(rows))
		for _, project := range rows {
			ownerID := project.GetString("owner_user_id")
			owner := owners[ownerID]
			if owner == nil {
				owner, _ = e.App.FindRecordById("users", ownerID)
				owners[ownerID] = owner
			}
			ownerName := ""
			if owner != nil {
				ownerName = publicPersonName(owner)
			}
			items = append(items, managedProject{
				ID:                 project.Id,
				ProjectNumber:      project.GetString("project_number"),
				Title:              project.GetString("title"),
				Summary:            project.GetString("summary"),
				Status:             project.GetString("execution_status"),
				Difficulty:         project.GetString("difficulty"),
				ReviewStatus:       project.GetString("review_status"),
				OwnerName:          ownerName,
				OwnerID:            ownerID,
				Created:            project.GetString("created"),
				Updated:            project.GetString("updated"),
				ProposedDifficulty: project.GetString("owner_proposed_difficulty"),
				OwnerConfirmed:     project.GetBool("owner_difficulty_confirmed"),
				ApproveCount:       0,
				OpposeCount:        0,
			})
			// Compute vote counts
			votes, _ := e.App.FindRecordsByFilter("project_difficulty_votes",
				"project_id = {:project}", "", 0, 0, dbx.Params{"project": project.Id})
			for _, v := range votes {
				if v.GetString("stance") == "approve" {
					items[len(items)-1].ApproveCount++
				} else {
					items[len(items)-1].OpposeCount++
				}
			}
		}
		return e.JSON(200, map[string]any{"items": items})
	})
}

func publicProjectVersionEditor(e *core.RequestEvent, project, version *core.Record) string {
	editor, err := e.App.FindRecordById("users", version.GetString("editor_id"))
	if err != nil {
		return ""
	}
	name := publicPersonName(editor)
	if profileSectionHidden(editor, "projects") || profileSectionHiddenForViewer(e.App, editor, "project:"+project.Id, e.Auth) {
		return maskedText(name)
	}
	return name
}

func validProjectDifficulty(value string) bool {
	for _, option := range []string{"D", "C", "B", "A", "S"} {
		if value == option {
			return true
		}
	}
	return false
}

func projectHallCard(e *core.RequestEvent, project *core.Record, users map[string]*core.Record) (publicProject, error) {
	card := projectSummary(e, project, users)
	display := func(id string) publicProjectMember {
		user := users[id]
		if user == nil {
			user, _ = e.App.FindRecordById("users", id)
			users[id] = user
		}
		if user == nil {
			return publicProjectMember{}
		}
		name := publicPersonName(user)
		hidden := profileSectionHidden(user, "projects") || profileSectionHiddenForViewer(e.App, user, "project:"+project.Id, e.Auth)
		if hidden {
			name = maskedText(name)
		}
		member := publicProjectMember{Name: name, Redacted: hidden}
		if !hidden && user.GetBool("profilePublic") && !profileSectionHidden(user, "avatar") && user.GetString("avatar") != "" {
			member.AvatarURL = contentURL("members", user.Id, "avatar")
		}
		return member
	}
	creator := display(project.GetString("creator_user_id"))
	card.CreatorName, card.CreatorRedacted, card.CreatorAvatarURL = creator.Name, creator.Redacted, creator.AvatarURL
	ownerID := project.GetString("owner_user_id")
	owner := display(ownerID)
	card.Managers = []publicProjectMember{owner}
	members, err := e.App.FindRecordsByFilter("project_members", `project_id = {:project} && status = "active"`, "", 0, 0, dbx.Params{"project": project.Id})
	if err != nil {
		return card, err
	}
	memberIDs := map[string]bool{ownerID: true}
	for _, member := range members {
		memberIDs[member.GetString("user_id")] = true
	}
	card.ParticipantCount = len(memberIDs)
	if project.GetBool("allow_others_management") {
		grants, err := e.App.FindRecordsByFilter("project_permission_grants", `project_id = {:project} && permission_code = "manage" && revoked_at = ""`, "", 0, 0, dbx.Params{"project": project.Id})
		if err != nil {
			return card, err
		}
		for _, grant := range grants {
			id := grant.GetString("user_id")
			if id != ownerID && memberIDs[id] {
				card.Managers = append(card.Managers, display(id))
			}
		}
	}
	return card, nil
}

func projectSummary(e *core.RequestEvent, project *core.Record, owners map[string]*core.Record) publicProject {
	ownerID := project.GetString("owner_user_id")
	owner := owners[ownerID]
	if owner == nil {
		owner, _ = e.App.FindRecordById("users", ownerID)
		owners[ownerID] = owner
	}
	ownerName := ""
	ownerAvatarURL := ""
	ownerRedacted := false
	if owner != nil {
		ownerName = publicPersonName(owner)
		ownerRedacted = profileSectionHidden(owner, "projects") || profileSectionHiddenForViewer(e.App, owner, "project:"+project.Id, e.Auth)
		if ownerRedacted {
			ownerName = maskedText(ownerName)
			ownerID = ""
		}
		if !ownerRedacted && owner.GetString("avatar") != "" {
			ownerAvatarURL = contentURL("members", owner.Id, "avatar")
		}
	}
	return publicProject{
		OwnerRedacted:  ownerRedacted,
		ID:             project.Id,
		ProjectNumber:  project.GetString("project_number"),
		Title:          project.GetString("title"),
		Sponsor:        project.GetString("sponsor"),
		Summary:        project.GetString("summary"),
		Status:         project.GetString("execution_status"),
		ProjectNature:  project.GetString("project_nature"),
		Difficulty:     project.GetString("difficulty"),
		DangerLevel:    project.GetString("danger_level"),
		TaskDifficulty: project.GetString("task_difficulty"),
		AdventureStyle: project.GetString("adventure_style"),
		OwnerID:        ownerID,
		OwnerName:      ownerName,
		OwnerAvatarURL: ownerAvatarURL,
		PublishedAt:    project.GetString("published_at"),
		CoverURL:       projectCoverURL(project),
		Updated:        project.GetString("updated"),
	}
}

func projectCoverURL(project *core.Record) string {
	if project.GetString("cover") == "" {
		return ""
	}
	return contentURL("projects", project.Id, "cover")
}

func projectFollowCounts(app core.App, projects []*core.Record) (map[string]int, error) {
	counts := make(map[string]int, len(projects))
	if len(projects) == 0 {
		return counts, nil
	}
	ids := make([]any, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.Id)
	}
	var rows []struct {
		ProjectID string `db:"project_id"`
		Total     int    `db:"total"`
	}
	err := app.DB().Select("project_id", "COUNT(*) AS total").From("project_follows").
		Where(dbx.HashExp{"is_active": true}).AndWhere(dbx.In("project_id", ids...)).GroupBy("project_id").All(&rows)
	for _, row := range rows {
		counts[row.ProjectID] = row.Total
	}
	return counts, err
}

func publicPersonName(record *core.Record) string {
	name := strings.TrimSpace(record.GetString("display_name"))
	if name == "" {
		name = strings.TrimSpace(record.GetString("name"))
	}
	return name
}

func projectDescriptionText(project *core.Record) string {
	var data map[string]any
	switch value := project.Get("description").(type) {
	case types.JSONRaw:
		_ = json.Unmarshal(value, &data)
	case string:
		_ = json.Unmarshal([]byte(value), &data)
	case map[string]any:
		data = value
	}
	if text, ok := data["text"].(string); ok {
		return text
	}
	return ""
}
