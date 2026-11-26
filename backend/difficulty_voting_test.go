package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDifficultyVoting(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	owner := publicProjectTestUser(t, app, "diff-owner@example.com", "负责人")
	admin1 := publicProjectTestUser(t, app, "diff-admin1@example.com", "管理员1")
	admin2 := publicProjectTestUser(t, app, "diff-admin2@example.com", "管理员2")
	admin3 := publicProjectTestUser(t, app, "diff-admin3@example.com", "管理员3")
	admin4 := publicProjectTestUser(t, app, "diff-admin4@example.com", "管理员4")
	for _, admin := range []*core.Record{admin1, admin2, admin3, admin4} {
		admin.Set("role", "admin")
		publicProjectTestSave(t, app, admin)
	}
	project := publicProjectTestProject(t, app, owner, "投票项目", "approved", "")

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindDifficultyVotingRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}

	tokenFor := func(user *core.Record) string {
		token, err := user.NewAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	// Owner proposes difficulty
	response := call("POST", "/api/hpahub/projects/"+project.Id+"/difficulty/propose", tokenFor(owner), `{"difficulty":"A"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("propose status = %d: %s", response.Code, response.Body.String())
	}
	saved, _ := app.FindRecordById("projects", project.Id)
	if saved.GetString("owner_proposed_difficulty") != "A" {
		t.Fatalf("proposed difficulty = %q", saved.GetString("owner_proposed_difficulty"))
	}

	// Admins vote to approve A
	for _, admin := range []*core.Record{admin1, admin2, admin3} {
		response = call("POST", "/api/hpahub/projects/"+project.Id+"/difficulty/vote", tokenFor(admin), `{"difficulty":"A","stance":"approve","comment":"同意"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("vote status = %d: %s", response.Code, response.Body.String())
		}
	}

	// Check status before owner confirms
	response = call("GET", "/api/hpahub/projects/"+project.Id+"/difficulty/status", tokenFor(owner), "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var status difficultyStatus
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.ApproveCounts["A"] != 3 || status.ConsensusReached {
		t.Fatalf("status before confirm: %+v", status)
	}

	// Owner confirms
	response = call("POST", "/api/hpahub/projects/"+project.Id+"/difficulty/confirm", tokenFor(owner), `{"confirmed":true,"proposedDifficulty":"A"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("confirm status = %d: %s", response.Code, response.Body.String())
	}
	saved, _ = app.FindRecordById("projects", project.Id)
	if saved.GetString("difficulty") != "A" {
		t.Fatalf("difficulty not updated after consensus: %q", saved.GetString("difficulty"))
	}

	// Check consensus reached
	response = call("GET", "/api/hpahub/projects/"+project.Id+"/difficulty/status", tokenFor(owner), "")
	json.Unmarshal(response.Body.Bytes(), &status)
	if !status.ConsensusReached || status.Consensus != "A" {
		t.Fatalf("consensus not reached: %+v", status)
	}

	// Admin4 opposes A - should break consensus
	response = call("POST", "/api/hpahub/projects/"+project.Id+"/difficulty/vote", tokenFor(admin4), `{"difficulty":"A","stance":"oppose","comment":"反对"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("oppose status = %d: %s", response.Code, response.Body.String())
	}
	saved, _ = app.FindRecordById("projects", project.Id)
	if saved.GetString("difficulty") != "" {
		t.Fatalf("difficulty should be cleared after opposition: %q", saved.GetString("difficulty"))
	}
	response = call("GET", "/api/hpahub/projects/"+project.Id+"/difficulty/status", tokenFor(owner), "")
	json.Unmarshal(response.Body.Bytes(), &status)
	if status.ConsensusReached {
		t.Fatalf("consensus should be broken after opposition: %+v", status)
	}

	// Non-admin cannot vote
	member := publicProjectTestUser(t, app, "diff-member@example.com", "成员")
	response = call("POST", "/api/hpahub/projects/"+project.Id+"/difficulty/vote", tokenFor(member), `{"difficulty":"A","stance":"approve"}`)
	if response.Code != http.StatusForbidden {
		t.Fatalf("member vote status = %d, want 403", response.Code)
	}

	// Completed project is frozen
	project.Set("execution_status", "completed")
	publicProjectTestSave(t, app, project)
	response = call("POST", "/api/hpahub/projects/"+project.Id+"/difficulty/vote", tokenFor(admin1), `{"difficulty":"A","stance":"approve"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("frozen vote status = %d, want 400", response.Code)
	}
}

func TestConsensusRequiresMatchingProposal(t *testing.T) {
	status := &difficultyStatus{ProposedDifficulty: "A", OwnerConfirmed: true, MinApprovals: 3, ApproveCounts: map[string]int{"B": 3}, OpposeCounts: map[string]int{}}
	if _, ok := computeDifficultyConsensus(status); ok {
		t.Fatal("accepted votes for a different difficulty")
	}
	status.ProposedDifficulty = "B"
	if level, ok := computeDifficultyConsensus(status); !ok || level != "B" {
		t.Fatalf("expected B consensus, got %q %v", level, ok)
	}
}
