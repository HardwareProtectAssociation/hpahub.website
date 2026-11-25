package migrations

// createProjects creates the project core tables. cover_asset_id and
// current_stage_id are added later by linkProjectAssets because they point at
// tables that do not exist yet (the document asks for the same two-step
// migration).
func (s *coCreation) createProjects() error {
	projects := coNew("projects",
		coRelation("creator_user_id", s.users.Id, true),
		coRelation("owner_user_id", s.users.Id, true),
		coRelation("responsible_admin_id", s.users.Id, false),
		coText("title", 160, true),
		coLongText("summary", false),
		coJSON("description", false),
		coSelect("review_status", true, "draft", "pending", "approved", "rejected"),
		coSelect("execution_status", true, "ongoing", "paused", "terminated", "completed"),
		coSelect("collaboration_mode", true, "independent", "collaborative"),
		coSelect("join_mode", true, "open", "approval"),
		coBool("accepts_sponsorship"),
		coNumber("estimated_duration_days", false, true, coFloat(1), coFloat(36500)),
		coDate("planned_start_date", false),
		coLongText("schedule_note", false),
		coDate("actual_started_at", false),
		coDate("actual_completed_at", false),
		coDate("submitted_at", false),
		coDate("published_at", false),
		coDate("deleted_at", false),
		coRelation("deleted_by", s.users.Id, false),
		coNumber("version", true, true, coFloat(1), coFloat(1<<31-1)),
		coSelect("adventure_style", true, "project", "dungeon", "boss"),
	)
	coIndexes(projects,
		"CREATE INDEX idx_projects_review_status ON projects (review_status, execution_status)",
		"CREATE INDEX idx_projects_creator ON projects (creator_user_id)",
		"CREATE INDEX idx_projects_owner ON projects (owner_user_id)",
		"CREATE INDEX idx_projects_responsible_admin ON projects (responsible_admin_id)",
	)
	if err := s.save(projects); err != nil {
		return err
	}
	s.projects = projects
	return nil
}

func (s *coCreation) createStages() error {
	stages := coNew("project_stages",
		coRelation("project_id", s.projects.Id, true),
		coText("name", 100, true),
		coLongText("description", false),
		coNumber("sort_order", false, true, nil, nil),
		coDate("started_at", false),
		coDate("completed_at", false),
	)
	coIndexes(stages,
		"CREATE INDEX idx_project_stages_project_order ON project_stages (project_id, sort_order)",
	)
	if err := s.save(stages); err != nil {
		return err
	}
	s.stages = stages
	return nil
}

func (s *coCreation) createMembership() error {
	members := coNew("project_members",
		coRelation("project_id", s.projects.Id, true),
		coRelation("user_id", s.users.Id, true),
		coLongText("duties", false),
		coSelect("status", true, "active", "left", "removed"),
		coDate("joined_at", true),
		coDate("left_at", false),
	)
	coIndexes(members,
		"CREATE UNIQUE INDEX idx_project_members_project_user ON project_members (project_id, user_id)",
		"CREATE INDEX idx_project_members_user_status ON project_members (user_id, status)",
	)
	if err := s.save(members); err != nil {
		return err
	}

	joinRequests := coNew("project_join_requests",
		coRelation("project_id", s.projects.Id, true),
		coRelation("user_id", s.users.Id, true),
		coSelect("source", true, "application", "invitation"),
		coRelation("inviter_user_id", s.users.Id, false),
		coLongText("message", false),
		coSelect("status", true, "pending", "accepted", "rejected", "withdrawn", "invalidated"),
		coRelation("handler_user_id", s.users.Id, false),
		coDate("handled_at", false),
		coLongText("handler_note", false),
	)
	coIndexes(joinRequests,
		"CREATE UNIQUE INDEX idx_project_join_requests_pending ON project_join_requests (project_id, user_id) WHERE status = 'pending'",
		"CREATE INDEX idx_project_join_requests_project_status ON project_join_requests (project_id, status, created)",
	)
	if err := s.save(joinRequests); err != nil {
		return err
	}

	grants := coNew("project_permission_grants",
		coRelation("project_id", s.projects.Id, true),
		coRelation("user_id", s.users.Id, true),
		coText("permission_code", 60, true),
		coRelation("granted_by", s.users.Id, true),
		coDate("granted_at", true),
		coDate("revoked_at", false),
	)
	coIndexes(grants,
		"CREATE UNIQUE INDEX idx_project_permission_grants_unique ON project_permission_grants (project_id, user_id, permission_code)",
		"CREATE INDEX idx_project_permission_grants_user ON project_permission_grants (user_id, project_id)",
	)
	if err := s.save(grants); err != nil {
		return err
	}

	permissionRequests := coNew("project_permission_requests",
		coRelation("project_id", s.projects.Id, true),
		coRelation("user_id", s.users.Id, true),
		coText("permission_code", 60, true),
		coLongText("reason", true),
		coSelect("status", true, "pending", "approved", "rejected", "withdrawn", "invalidated"),
		coRelation("reviewer_user_id", s.users.Id, false),
		coLongText("review_note", false),
		coDate("reviewed_at", false),
	)
	coIndexes(permissionRequests,
		"CREATE UNIQUE INDEX idx_project_permission_requests_pending ON project_permission_requests (project_id, user_id, permission_code) WHERE status = 'pending'",
		"CREATE INDEX idx_project_permission_requests_project_status ON project_permission_requests (project_id, status)",
	)
	return s.save(permissionRequests)
}

func (s *coCreation) createShowcases() error {
	showcases := coNew("user_showcases",
		coRelation("user_id", s.users.Id, true),
		coRelation("project_id", s.projects.Id, false),
		coText("external_url", 500, false),
		coText("title", 160, true),
		coLongText("description", false),
		coLongText("contribution_note", false),
		coText("cover_url", 500, false),
		coNumber("sort_order", false, true, nil, nil),
	)
	coIndexes(showcases,
		"CREATE INDEX idx_user_showcases_user_order ON user_showcases (user_id, sort_order)",
	)
	return s.save(showcases)
}
