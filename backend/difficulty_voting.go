package main

import (
	"database/sql"
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type difficultyVote struct {
	ID         string `json:"id"`
	Voter      string `json:"voterName"`
	VoterID    string `json:"voterId"`
	Difficulty string `json:"difficulty"`
	Stance     string `json:"stance"`
	Comment    string `json:"comment"`
	Created    string `json:"created"`
}

type difficultyStatus struct {
	ProposedDifficulty string           `json:"proposedDifficulty"`
	OwnerConfirmed     bool             `json:"ownerConfirmed"`
	Consensus          string           `json:"consensus"`
	ConsensusReached   bool             `json:"consensusReached"`
	Frozen             bool             `json:"frozen"`
	ApproveCounts      map[string]int   `json:"approveCounts"`
	OpposeCounts       map[string]int   `json:"opposeCounts"`
	Votes              []difficultyVote `json:"votes"`
	MinApprovals       int              `json:"minApprovals"`
}

func bindDifficultyVotingRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/hpahub/projects/{id}/difficulty/status", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		role := siteRole(e.Auth)
		if role != "admin" && role != "octopus" {
			// Owner can also view their project's difficulty status.
			project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
			if err != nil {
				return e.NotFoundError("Not found.", nil)
			}
			owner, member, err := personalCenterOwner(e.App, e.Auth)
			if err != nil || !member || project.GetString("owner_user_id") != owner.Id {
				return e.ForbiddenError("Insufficient permissions.", nil)
			}
		}
		status, err := difficultyStatusForProject(e.App, e.Request.PathValue("id"))
		if err != nil {
			return e.InternalServerError("Unable to load difficulty status.", err)
		}
		return e.JSON(200, status)
	})

	e.Router.POST("/api/hpahub/projects/{id}/difficulty/vote", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		role := siteRole(e.Auth)
		if role != "admin" && role != "octopus" {
			return e.ForbiddenError("Only administrators can vote on difficulty.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("execution_status") == "completed" {
			return e.BadRequestError("Project is completed; difficulty is frozen.", nil)
		}
		voter, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !member {
			return e.InternalServerError("Unable to identify voter.", err)
		}
		var input struct {
			Difficulty string `json:"difficulty"`
			Stance     string `json:"stance"`
			Comment    string `json:"comment"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid difficulty vote.", err)
		}
		if !validProjectDifficulty(input.Difficulty) || (input.Stance != "approve" && input.Stance != "oppose") {
			return e.BadRequestError("Invalid difficulty vote.", nil)
		}
		if len(input.Comment) > 500 {
			return e.BadRequestError("Comment too long.", nil)
		}
		existing, err := e.App.FindFirstRecordByFilter("project_difficulty_votes",
			"project_id = {:project} && voter_user_id = {:voter}", dbx.Params{"project": project.Id, "voter": voter.Id})
		if errors.Is(err, sql.ErrNoRows) {
			collection, err := e.App.FindCollectionByNameOrId("project_difficulty_votes")
			if err != nil {
				return e.InternalServerError("Unable to save vote.", err)
			}
			existing = core.NewRecord(collection)
			existing.Set("project_id", project.Id)
			existing.Set("voter_user_id", voter.Id)
		} else if err != nil {
			return e.InternalServerError("Unable to save vote.", err)
		}
		existing.Set("difficulty", input.Difficulty)
		existing.Set("stance", input.Stance)
		existing.Set("comment", input.Comment)
		if err := e.App.Save(existing); err != nil {
			return e.InternalServerError("Unable to save vote.", err)
		}
		if err := updateProjectDifficultyConsensus(e.App, project); err != nil {
			return e.InternalServerError("Unable to update difficulty.", err)
		}
		status, err := difficultyStatusForProject(e.App, project.Id)
		if err != nil {
			return e.InternalServerError("Unable to load difficulty status.", err)
		}
		return e.JSON(200, status)
	})

	e.Router.POST("/api/hpahub/projects/{id}/difficulty/confirm", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("execution_status") == "completed" {
			return e.BadRequestError("Project is completed; difficulty is frozen.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !member {
			return e.ForbiddenError("Only the project owner can confirm.", nil)
		}
		if project.GetString("owner_user_id") != owner.Id {
			return e.ForbiddenError("Only the project owner can confirm.", nil)
		}
		var input struct {
			Confirmed bool   `json:"confirmed"`
			Proposed  string `json:"proposedDifficulty"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid confirmation.", err)
		}
		if input.Proposed != "" && !validProjectDifficulty(input.Proposed) {
			return e.BadRequestError("Invalid proposed difficulty.", nil)
		}
		project.Set("owner_difficulty_confirmed", input.Confirmed)
		if input.Proposed != "" {
			project.Set("owner_proposed_difficulty", input.Proposed)
		}
		if err := e.App.Save(project); err != nil {
			return e.InternalServerError("Unable to save confirmation.", err)
		}
		if err := updateProjectDifficultyConsensus(e.App, project); err != nil {
			return e.InternalServerError("Unable to update difficulty.", err)
		}
		status, err := difficultyStatusForProject(e.App, project.Id)
		if err != nil {
			return e.InternalServerError("Unable to load difficulty status.", err)
		}
		return e.JSON(200, status)
	})

	e.Router.POST("/api/hpahub/projects/{id}/difficulty/propose", func(e *core.RequestEvent) error {
		noCache(e)
		if e.Auth == nil {
			return e.UnauthorizedError("Authentication required.", nil)
		}
		project, err := e.App.FindRecordById("projects", e.Request.PathValue("id"))
		if err != nil || project.GetString("deleted_at") != "" {
			return e.NotFoundError("Not found.", nil)
		}
		if project.GetString("execution_status") == "completed" {
			return e.BadRequestError("Project is completed; difficulty is frozen.", nil)
		}
		owner, member, err := personalCenterOwner(e.App, e.Auth)
		if err != nil || !member {
			return e.ForbiddenError("Only the project owner can propose.", nil)
		}
		if project.GetString("owner_user_id") != owner.Id {
			return e.ForbiddenError("Only the project owner can propose.", nil)
		}
		var input struct {
			Difficulty string `json:"difficulty"`
		}
		if err := e.BindBody(&input); err != nil {
			return e.BadRequestError("Invalid proposal.", err)
		}
		if !validProjectDifficulty(input.Difficulty) {
			return e.BadRequestError("Invalid proposed difficulty.", nil)
		}
		project.Set("owner_proposed_difficulty", input.Difficulty)
		project.Set("owner_difficulty_confirmed", false) // Reset confirmation when proposing new difficulty
		if err := e.App.Save(project); err != nil {
			return e.InternalServerError("Unable to save proposal.", err)
		}
		if err := updateProjectDifficultyConsensus(e.App, project); err != nil {
			return e.InternalServerError("Unable to update difficulty.", err)
		}
		status, err := difficultyStatusForProject(e.App, project.Id)
		if err != nil {
			return e.InternalServerError("Unable to load difficulty status.", err)
		}
		return e.JSON(200, status)
	})
}

func difficultyStatusForProject(app core.App, projectID string) (*difficultyStatus, error) {
	project, err := app.FindRecordById("projects", projectID)
	if err != nil {
		return nil, err
	}
	status := &difficultyStatus{
		ProposedDifficulty: project.GetString("owner_proposed_difficulty"),
		OwnerConfirmed:     project.GetBool("owner_difficulty_confirmed"),
		Frozen:             project.GetString("execution_status") == "completed",
		ApproveCounts:      map[string]int{},
		OpposeCounts:       map[string]int{},
		Votes:              []difficultyVote{},
		MinApprovals:       3,
	}

	votes, err := app.FindRecordsByFilter("project_difficulty_votes",
		"project_id = {:project}", "-created", 0, 0, dbx.Params{"project": projectID})
	if err != nil {
		return nil, err
	}
	for _, vote := range votes {
		diff := vote.GetString("difficulty")
		if vote.GetString("stance") == "approve" {
			status.ApproveCounts[diff]++
		} else {
			status.OpposeCounts[diff]++
		}
		voterName := ""
		if voter, err := app.FindRecordById("users", vote.GetString("voter_user_id")); err == nil {
			voterName = publicPersonName(voter)
		}
		status.Votes = append(status.Votes, difficultyVote{
			ID:         vote.Id,
			Voter:      voterName,
			VoterID:    vote.GetString("voter_user_id"),
			Difficulty: diff,
			Stance:     vote.GetString("stance"),
			Comment:    vote.GetString("comment"),
			Created:    vote.GetString("created"),
		})
	}

	status.Consensus, status.ConsensusReached = computeDifficultyConsensus(status)
	return status, nil
}

// computeDifficultyConsensus returns the consensus difficulty level and whether
// consensus is reached. Consensus requires >=3 approvals, 0 opposition, and
// only one candidate level.
func computeDifficultyConsensus(status *difficultyStatus) (string, bool) {
	levels := []string{"S", "A", "B", "C", "D"}
	candidates := []string{}
	for _, level := range levels {
		if status.ApproveCounts[level] >= status.MinApprovals && status.OpposeCounts[level] == 0 {
			candidates = append(candidates, level)
		}
	}
	if len(candidates) != 1 {
		return "", false
	}
	// Owner must also agree (confirm).
	if !status.OwnerConfirmed || status.ProposedDifficulty != candidates[0] {
		return "", false
	}
	return candidates[0], true
}

func updateProjectDifficultyConsensus(app core.App, project *core.Record) error {
	status, err := difficultyStatusForProject(app, project.Id)
	if err != nil {
		return err
	}
	if project.GetString("difficulty") != status.Consensus {
		project.Set("difficulty", status.Consensus)
		if err := app.Save(project); err != nil {
			return err
		}
	}
	return nil
}
