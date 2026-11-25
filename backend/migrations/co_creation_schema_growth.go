package migrations

func (s *coCreation) createTasks() error {
	tasks := coNew("project_tasks",
		coRelation("project_id", s.projects.Id, true),
		coRelation("creator_user_id", s.users.Id, true),
		coText("title", 160, true),
		coLongText("description", true),
		coRelation("stage_id", s.stages.Id, false),
		coRelation("part_content_id", s.contents.Id, false),
		coSelect("status", true, "open", "in_progress", "submitted", "completed", "cancelled"),
		coRelation("accepted_by", s.users.Id, false),
		coDate("accepted_at", false),
	)
	coIndexes(tasks,
		"CREATE INDEX idx_project_tasks_project_status ON project_tasks (project_id, status)",
	)
	if err := s.save(tasks); err != nil {
		return err
	}
	s.tasks = tasks
	return nil
}

func (s *coCreation) createParticipantRoles() error {
	roles := coNew("project_participant_roles",
		coRelation("project_id", s.projects.Id, true),
		coRelation("user_id", s.users.Id, true),
		coRelation("identity_id", s.identities.Id, true),
		coLongText("duties", false),
		coSelect("status", true, "pending", "active", "inactive"),
		coRelation("assigned_by", s.users.Id, true),
		coRelation("confirmed_by", s.users.Id, false),
		coDate("confirmed_at", false),
	)
	coIndexes(roles,
		"CREATE UNIQUE INDEX idx_project_participant_roles_unique ON project_participant_roles (project_id, user_id, identity_id)",
		"CREATE INDEX idx_project_participant_roles_user_status ON project_participant_roles (user_id, status)",
	)
	if err := s.save(roles); err != nil {
		return err
	}
	s.roles = roles
	return nil
}

func (s *coCreation) createTaskContributions() error {
	contributions := coNew("project_task_contributions",
		coRelation("task_id", s.tasks.Id, true),
		coRelation("user_id", s.users.Id, true),
		coRelation("career_identity_id", s.identities.Id, true),
		coLongText("contribution_note", true),
		coRelation("evidence_content_id", s.contents.Id, false),
		coNumber("allocation_basis_points", false, true, coFloat(0), coFloat(10000)),
		coSelect("status", true, "submitted", "confirmed", "rejected"),
		coRelation("confirmed_by", s.users.Id, false),
		coDate("confirmed_at", false),
		coRelation("participant_role_id", s.roles.Id, true),
	)
	coIndexes(contributions,
		"CREATE UNIQUE INDEX idx_project_task_contributions_unique ON project_task_contributions (task_id, user_id, career_identity_id)",
		"CREATE INDEX idx_project_task_contributions_user_career ON project_task_contributions (user_id, career_identity_id, status)",
	)
	if err := s.save(contributions); err != nil {
		return err
	}
	s.contributions = contributions
	return nil
}

func (s *coCreation) createXpVoting() error {
	ballots := coNew("task_xp_ballots",
		coRelation("task_id", s.tasks.Id, true),
		coNumber("round_number", true, true, coFloat(1), nil),
		coRelation("opened_by", s.users.Id, true),
		coSelect("status", true, "open", "closed", "void"),
		coJSON("rules_snapshot", true),
		coDate("opens_at", true),
		coDate("closes_at", true),
		coNumber("eligible_count", false, true, coFloat(0), nil),
		coNumber("valid_vote_count", false, true, coFloat(0), nil),
		coNumber("final_xp", false, true, coFloat(0), nil),
		coDate("closed_at", false),
	)
	coIndexes(ballots,
		"CREATE UNIQUE INDEX idx_task_xp_ballots_task_round ON task_xp_ballots (task_id, round_number)",
		"CREATE INDEX idx_task_xp_ballots_status_closes ON task_xp_ballots (status, closes_at)",
	)
	if err := s.save(ballots); err != nil {
		return err
	}
	s.ballots = ballots

	votes := coNew("task_xp_votes",
		coRelation("ballot_id", ballots.Id, true),
		coRelation("user_id", s.users.Id, true),
		coDate("eligible_at", true),
		coNumber("xp_value", false, true, coFloat(0), nil),
		coDate("voted_at", false),
	)
	coIndexes(votes,
		"CREATE UNIQUE INDEX idx_task_xp_votes_ballot_user ON task_xp_votes (ballot_id, user_id)",
	)
	return s.save(votes)
}

// createCareerLedger creates the append-only XP ledger and the per-career level
// thresholds. reverses_entry_id is a self relation added in a second save.
func (s *coCreation) createCareerLedger() error {
	ledger := coNew("career_xp_ledger",
		coRelation("user_id", s.users.Id, true),
		coRelation("career_identity_id", s.identities.Id, true),
		coRelation("contribution_id", s.contributions.Id, true),
		coRelation("ballot_id", s.ballots.Id, true),
		coSelect("entry_kind", true, "award", "reversal"),
		coNumber("xp_delta", true, true, nil, nil),
		coText("business_key", 200, true),
		coJSON("allocation_snapshot", true),
		coLongText("reason", false),
	)
	coIndexes(ledger,
		"CREATE UNIQUE INDEX idx_career_xp_ledger_business_key ON career_xp_ledger (business_key)",
		// A contribution may only be awarded once; reversal entries stay append-only.
		"CREATE UNIQUE INDEX idx_career_xp_ledger_contribution_award ON career_xp_ledger (contribution_id) WHERE entry_kind = 'award'",
		"CREATE INDEX idx_career_xp_ledger_user_career ON career_xp_ledger (user_id, career_identity_id, created)",
	)
	if err := s.save(ledger); err != nil {
		return err
	}
	ledger.Fields.Add(coRelation("reverses_entry_id", ledger.Id, false))
	if err := s.save(ledger); err != nil {
		return err
	}

	thresholds := coNew("career_level_thresholds",
		coRelation("career_identity_id", s.identities.Id, true),
		coNumber("level_number", true, true, coFloat(1), nil),
		coNumber("minimum_total_xp", false, true, coFloat(0), nil),
		coText("display_title", 80, false),
	)
	coIndexes(thresholds,
		"CREATE UNIQUE INDEX idx_career_level_thresholds_identity_level ON career_level_thresholds (career_identity_id, level_number)",
	)
	if err := s.save(thresholds); err != nil {
		return err
	}
	s.thresholds = thresholds
	return nil
}

func (s *coCreation) createCompletionRatings() error {
	ratings := coNew("project_completion_ratings",
		coRelation("project_id", s.projects.Id, true),
		coRelation("user_id", s.users.Id, true),
		coNumber("score", true, true, coFloat(1), coFloat(5)),
		coLongText("review_text", false),
		coSelect("status", true, "visible", "hidden"),
	)
	coIndexes(ratings,
		"CREATE UNIQUE INDEX idx_project_completion_ratings_project_user ON project_completion_ratings (project_id, user_id)",
	)
	return s.save(ratings)
}
