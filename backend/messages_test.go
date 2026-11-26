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

func TestMessaging(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	alice := centerTestUser(t, app, "msg-alice@example.com", "member")
	bob := centerTestUser(t, app, "msg-bob@example.com", "member")
	aliceToken, _ := alice.NewAuthToken()
	bobToken, _ := bob.NewAuthToken()

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	bindMessageRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}

	// Alice sends message to Bob
	response := call("POST", "/api/hpahub/me/messages", aliceToken,
		`{"recipientId":"`+bob.Id+`","subject":"你好","content":"想和你聊聊项目合作。"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("send status = %d: %s", response.Code, response.Body.String())
	}

	// Bob sees the message
	response = call("GET", "/api/hpahub/me/messages", bobToken, "")
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", response.Code, response.Body.String())
	}
	var list messageList
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.UnreadCount != 1 || list.Items[0].Subject != "你好" {
		t.Fatalf("messages: %+v", list)
	}

	// Bob marks as read
	response = call("POST", "/api/hpahub/me/messages/"+list.Items[0].ID+"/read", bobToken, "")
	if response.Code != http.StatusOK {
		t.Fatalf("read status = %d: %s", response.Code, response.Body.String())
	}
	response = call("GET", "/api/hpahub/me/messages/unread-count", bobToken, "")
	var count struct {
		UnreadCount int `json:"unreadCount"`
	}
	json.Unmarshal(response.Body.Bytes(), &count)
	if count.UnreadCount != 0 {
		t.Fatalf("unread count = %d", count.UnreadCount)
	}

	// Cannot send to self
	response = call("POST", "/api/hpahub/me/messages", aliceToken,
		`{"recipientId":"`+alice.Id+`","subject":"test","content":"self"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("self message status = %d, want 400", response.Code)
	}

	// System message
	if err := sendSystemMessage(app, bob.Id, "申请已通过", "你的申请已通过。", "project_approval", ""); err != nil {
		t.Fatal(err)
	}
	response = call("GET", "/api/hpahub/me/messages", bobToken, "")
	json.Unmarshal(response.Body.Bytes(), &list)
	if len(list.Items) != 2 || list.Items[0].Category != "project_approval" {
		t.Fatalf("system message: %+v", list.Items)
	}

	// Alice cannot read Bob's messages
	response = call("GET", "/api/hpahub/me/messages", aliceToken, "")
	json.Unmarshal(response.Body.Bytes(), &list)
	if len(list.Items) != 0 {
		t.Fatalf("alice sees bob's messages: %+v", list.Items)
	}
}
