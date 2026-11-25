package migrations

// createSponsorshipDefinitions creates the reward and sponsorship-need
// definitions. Sponsorship responses are created in the next step.
func (s *coCreation) createSponsorshipDefinitions() error {
	needs := coNew("project_sponsorship_needs",
		coRelation("project_id", s.projects.Id, true),
		coRelation("creator_user_id", s.users.Id, true),
		coText("title", 120, true),
		coSelect("sponsorship_type", true, "technical", "funding", "tools", "materials", "space", "other"),
		coLongText("description", true),
		// Money is stored as integer minor units because PocketBase/SQLite has
		// no exact decimal type; see README.md.
		coNumber("target_amount_cents", false, true, coFloat(0), nil),
		coText("currency", 3, false),
		coNumber("target_quantity", false, false, coFloat(0), nil),
		coText("unit", 30, false),
		coSelect("status", true, "open", "fulfilled", "closed"),
	)
	coIndexes(needs,
		"CREATE INDEX idx_project_sponsorship_needs_project_status ON project_sponsorship_needs (project_id, status)",
	)
	if err := s.save(needs); err != nil {
		return err
	}
	s.needs = needs

	rewards := coNew("project_rewards",
		coRelation("project_id", s.projects.Id, true),
		coRelation("creator_user_id", s.users.Id, true),
		coText("title", 120, true),
		coSelect("reward_type", true, "money", "product", "title", "credit", "learning", "other"),
		coLongText("description", false),
		coLongText("conditions", true),
		coNumber("quantity_limit", false, true, coFloat(1), nil),
		coSelect("status", true, "active", "closed"),
	)
	coIndexes(rewards,
		"CREATE INDEX idx_project_rewards_project_status ON project_rewards (project_id, status)",
	)
	if err := s.save(rewards); err != nil {
		return err
	}
	s.rewards = rewards
	return nil
}

func (s *coCreation) createSponsorships() error {
	sponsorships := coNew("project_sponsorships",
		coRelation("need_id", s.needs.Id, true),
		coRelation("sponsor_user_id", s.users.Id, true),
		coLongText("offered_description", true),
		coNumber("pledged_amount_cents", false, true, coFloat(0), nil),
		coNumber("pledged_quantity", false, false, coFloat(0), nil),
		coNumber("delivered_amount_cents", false, true, coFloat(0), nil),
		coNumber("delivered_quantity", false, false, coFloat(0), nil),
		coSelect("status", true, "proposed", "accepted", "fulfilling", "provided", "confirmed", "withdrawn", "rejected"),
		coLongText("fulfillment_note", false),
		coRelation("confirmed_by", s.users.Id, false),
		coDate("confirmed_at", false),
	)
	coIndexes(sponsorships,
		"CREATE INDEX idx_project_sponsorships_need_status ON project_sponsorships (need_id, status)",
		"CREATE INDEX idx_project_sponsorships_sponsor_status ON project_sponsorships (sponsor_user_id, status)",
	)
	if err := s.save(sponsorships); err != nil {
		return err
	}
	s.sponsorships = sponsorships
	return nil
}

// createContents creates the unified project content table. parent_content_id
// is a self relation and is therefore added in a second save.
func (s *coCreation) createContents() error {
	contents := coNew("project_contents",
		coRelation("project_id", s.projects.Id, true),
		coRelation("author_user_id", s.users.Id, true),
		coSelect("type", true, "part", "update", "question", "result"),
		coRelation("stage_id", s.stages.Id, false),
		coRelation("sponsorship_id", s.sponsorships.Id, false),
		coText("title", 160, true),
		coJSON("body", false),
		coSelect("status", true, "draft", "published", "hidden"),
		coBool("rating_enabled"),
		coBool("is_pinned"),
		coDate("published_at", false),
	)
	coIndexes(contents,
		"CREATE INDEX idx_project_contents_project_type ON project_contents (project_id, type, published_at)",
		"CREATE INDEX idx_project_contents_author ON project_contents (author_user_id)",
	)
	if err := s.save(contents); err != nil {
		return err
	}
	contents.Fields.Add(coRelation("parent_content_id", contents.Id, false))
	if err := s.save(contents); err != nil {
		return err
	}
	s.contents = contents
	return nil
}

func (s *coCreation) createMediaAssets() error {
	media := coNew("media_assets",
		coRelation("uploader_user_id", s.users.Id, true),
		coRelation("project_id", s.projects.Id, false),
		coRelation("content_id", s.contents.Id, false),
		coText("storage_key", 500, true),
		coText("original_name", 255, true),
		coText("mime_type", 100, true),
		coNumber("size_bytes", true, true, coFloat(1), coFloat(1<<40)),
		coLongText("alt_text", false),
	)
	coIndexes(media,
		"CREATE INDEX idx_media_assets_project ON media_assets (project_id)",
		"CREATE INDEX idx_media_assets_content ON media_assets (content_id)",
		"CREATE INDEX idx_media_assets_uploader ON media_assets (uploader_user_id)",
	)
	return s.save(media)
}

// linkProjectAssets adds the two optional reverse references once stages and
// media assets exist.
func (s *coCreation) linkProjectAssets() error {
	media, err := s.app.FindCollectionByNameOrId("media_assets")
	if err != nil {
		return err
	}
	s.projects.Fields.Add(
		coRelation("cover_asset_id", media.Id, false),
		coRelation("current_stage_id", s.stages.Id, false),
	)
	return s.save(s.projects)
}

func (s *coCreation) createInteractions() error {
	comments := coNew("project_comments",
		coRelation("content_id", s.contents.Id, true),
		coRelation("author_user_id", s.users.Id, true),
		coLongText("body", true),
		coSelect("status", true, "visible", "hidden", "deleted"),
	)
	coIndexes(comments,
		"CREATE INDEX idx_project_comments_content_created ON project_comments (content_id, created)",
		"CREATE INDEX idx_project_comments_author ON project_comments (author_user_id)",
	)
	if err := s.save(comments); err != nil {
		return err
	}
	comments.Fields.Add(coRelation("parent_comment_id", comments.Id, false))
	if err := s.save(comments); err != nil {
		return err
	}
	s.comments = comments

	ratings := coNew("project_ratings",
		coRelation("content_id", s.contents.Id, true),
		coRelation("user_id", s.users.Id, true),
		coNumber("score", true, true, coFloat(1), coFloat(5)),
		coLongText("review_text", false),
		coSelect("status", true, "visible", "hidden"),
	)
	coIndexes(ratings,
		"CREATE UNIQUE INDEX idx_project_ratings_content_user ON project_ratings (content_id, user_id)",
	)
	if err := s.save(ratings); err != nil {
		return err
	}

	follows := coNew("project_follows",
		coRelation("project_id", s.projects.Id, true),
		coRelation("user_id", s.users.Id, true),
		coBool("is_active"),
		coBool("receive_updates"),
		coDate("followed_at", true),
	)
	coIndexes(follows,
		"CREATE UNIQUE INDEX idx_project_follows_project_user ON project_follows (project_id, user_id)",
		"CREATE INDEX idx_project_follows_project_active ON project_follows (project_id, is_active)",
	)
	return s.save(follows)
}
