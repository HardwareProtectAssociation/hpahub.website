package migrations

// createAgreements creates cooperation agreements and reward fulfillments.
// supersedes_id is a self relation and is added in a second save.
func (s *coCreation) createAgreements() error {
	agreements := coNew("project_agreements",
		coRelation("project_id", s.projects.Id, true),
		coRelation("participant_user_id", s.users.Id, true),
		coRelation("proposer_user_id", s.users.Id, true),
		coJSON("terms_snapshot", true),
		coSelect("status", true, "proposed", "confirmed", "rejected", "superseded"),
		coDate("participant_confirmed_at", false),
		coRelation("project_confirmed_by", s.users.Id, false),
		coDate("project_confirmed_at", false),
	)
	coIndexes(agreements,
		"CREATE INDEX idx_project_agreements_project_participant ON project_agreements (project_id, participant_user_id, status)",
	)
	if err := s.save(agreements); err != nil {
		return err
	}
	agreements.Fields.Add(coRelation("supersedes_id", agreements.Id, false))
	if err := s.save(agreements); err != nil {
		return err
	}
	s.agreements = agreements

	fulfillments := coNew("project_reward_fulfillments",
		coRelation("reward_id", s.rewards.Id, true),
		coRelation("recipient_user_id", s.users.Id, true),
		coRelation("agreement_id", agreements.Id, false),
		coJSON("reward_snapshot", true),
		coNumber("quantity", true, true, coFloat(1), nil),
		coSelect("status", true, "promised", "eligible", "issued", "received", "cancelled"),
		coRelation("issued_by", s.users.Id, false),
		coDate("issued_at", false),
		coDate("received_at", false),
	)
	coIndexes(fulfillments,
		"CREATE INDEX idx_project_reward_fulfillments_reward_status ON project_reward_fulfillments (reward_id, status)",
		"CREATE INDEX idx_project_reward_fulfillments_recipient ON project_reward_fulfillments (recipient_user_id, status)",
	)
	return s.save(fulfillments)
}

func (s *coCreation) createUnions() error {
	unions := coNew("project_unions",
		coRelation("creator_user_id", s.users.Id, true),
		coText("title", 160, true),
		coJSON("description", false),
		coSelect("status", true, "active", "archived"),
	)
	coIndexes(unions, "CREATE INDEX idx_project_unions_creator ON project_unions (creator_user_id)")
	if err := s.save(unions); err != nil {
		return err
	}

	members := coNew("project_union_members",
		coRelation("union_id", unions.Id, true),
		coRelation("project_id", s.projects.Id, true),
		coRelation("invited_by", s.users.Id, true),
		coSelect("status", true, "invited", "accepted", "rejected", "left"),
		coRelation("confirmed_by", s.users.Id, false),
		coDate("confirmed_at", false),
		coDate("left_at", false),
	)
	coIndexes(members,
		"CREATE UNIQUE INDEX idx_project_union_members_union_project ON project_union_members (union_id, project_id)",
		"CREATE INDEX idx_project_union_members_project_status ON project_union_members (project_id, status)",
	)
	return s.save(members)
}

func (s *coCreation) createSkillNeeds() error {
	needs := coNew("project_skill_needs",
		coRelation("project_id", s.projects.Id, true),
		coRelation("skill_id", s.skills.Id, false),
		coText("help_type", 40, false),
		coLongText("work_description", true),
		coText("expected_proficiency", 20, false),
		coNumber("expected_weekly_hours", false, false, coFloat(0), coFloat(168)),
		coNumber("people_needed", false, true, coFloat(1), nil),
		coSelect("status", true, "open", "closed"),
	)
	coIndexes(needs,
		"CREATE INDEX idx_project_skill_needs_project_status_skill ON project_skill_needs (project_id, status, skill_id)",
	)
	return s.save(needs)
}

func (s *coCreation) createGovernanceQueues() error {
	reviews := coNew("project_review_records",
		coRelation("project_id", s.projects.Id, true),
		coRelation("reviewer_user_id", s.users.Id, true),
		coSelect("decision", true, "approved", "rejected"),
		coLongText("note", false),
		coDate("reviewed_at", true),
	)
	coIndexes(reviews,
		"CREATE INDEX idx_project_review_records_project ON project_review_records (project_id, reviewed_at)",
	)
	if err := s.save(reviews); err != nil {
		return err
	}

	takeovers := coNew("project_takeover_requests",
		coRelation("project_id", s.projects.Id, true),
		coRelation("applicant_user_id", s.users.Id, true),
		coRelation("previous_owner_user_id", s.users.Id, true),
		coLongText("reason", true),
		coLongText("continuation_plan", true),
		coJSON("handover_snapshot", false),
		coDate("previous_owner_confirmed_at", false),
		coDate("applicant_confirmed_at", false),
		coSelect("status", true, "pending", "approved", "rejected", "withdrawn", "invalidated"),
		coRelation("reviewer_user_id", s.users.Id, false),
		coLongText("review_note", false),
		coDate("reviewed_at", false),
		coDate("transferred_at", false),
	)
	coIndexes(takeovers,
		"CREATE UNIQUE INDEX idx_project_takeover_requests_pending ON project_takeover_requests (project_id, applicant_user_id) WHERE status = 'pending'",
		"CREATE INDEX idx_project_takeover_requests_project_status ON project_takeover_requests (project_id, status)",
	)
	if err := s.save(takeovers); err != nil {
		return err
	}

	deletions := coNew("project_deletion_requests",
		coRelation("project_id", s.projects.Id, true),
		coRelation("applicant_user_id", s.users.Id, true),
		coLongText("reason", true),
		coSelect("status", true, "pending", "approved", "rejected", "withdrawn"),
		coRelation("reviewer_user_id", s.users.Id, false),
		coLongText("review_note", false),
		coDate("reviewed_at", false),
	)
	coIndexes(deletions,
		"CREATE UNIQUE INDEX idx_project_deletion_requests_pending ON project_deletion_requests (project_id) WHERE status = 'pending'",
		"CREATE INDEX idx_project_deletion_requests_status ON project_deletion_requests (status, created)",
	)
	return s.save(deletions)
}

func (s *coCreation) createPlatformRecords() error {
	notifications := coNew("notifications",
		coRelation("recipient_user_id", s.users.Id, true),
		coRelation("actor_user_id", s.users.Id, false),
		coRelation("project_id", s.projects.Id, false),
		coText("type", 40, true),
		coText("target_type", 40, true),
		coText("target_id", 64, false),
		coJSON("payload", false),
		coText("event_key", 200, true),
		coDate("read_at", false),
	)
	coIndexes(notifications,
		"CREATE UNIQUE INDEX idx_notifications_recipient_event ON notifications (recipient_user_id, event_key)",
		"CREATE INDEX idx_notifications_recipient_read ON notifications (recipient_user_id, read_at, created)",
	)
	if err := s.save(notifications); err != nil {
		return err
	}

	audits := coNew("audit_events",
		coRelation("project_id", s.projects.Id, false),
		coRelation("actor_user_id", s.users.Id, false),
		coText("action", 80, true),
		coText("target_type", 40, true),
		coText("target_id", 64, false),
		coJSON("before_data", false),
		coJSON("after_data", false),
		coLongText("reason", false),
	)
	coIndexes(audits,
		"CREATE INDEX idx_audit_events_project_created ON audit_events (project_id, created)",
		"CREATE INDEX idx_audit_events_actor_created ON audit_events (actor_user_id, created)",
	)
	if err := s.save(audits); err != nil {
		return err
	}

	reports := coNew("reports",
		coRelation("reporter_user_id", s.users.Id, true),
		coRelation("project_id", s.projects.Id, false),
		coRelation("content_id", s.contents.Id, false),
		coRelation("comment_id", s.comments.Id, false),
		coLongText("reason", true),
		coSelect("status", true, "pending", "resolved", "dismissed"),
		coRelation("handler_user_id", s.users.Id, false),
		coLongText("handler_note", false),
		coDate("handled_at", false),
	)
	coIndexes(reports,
		"CREATE INDEX idx_reports_status_created ON reports (status, created)",
		"CREATE INDEX idx_reports_project ON reports (project_id)",
		"CREATE INDEX idx_reports_content ON reports (content_id)",
		"CREATE INDEX idx_reports_comment ON reports (comment_id)",
	)
	return s.save(reports)
}
