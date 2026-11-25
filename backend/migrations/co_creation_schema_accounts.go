package migrations

// extendUsers adds the co-creation account fields to the existing PocketBase
// auth collection. The document's users table maps as follows:
//
//	id                 -> PocketBase id
//	email              -> email
//	password_hash      -> password
//	avatar_url         -> avatar (file field added by migration 1760000002)
//	bio                -> bio (added by migration 1760000002)
//	platform_role      -> existing role select (member/user = user, admin = admin)
//	account_status     -> account_status (backfilled to active)
//	registration_source-> registration_source (existing accounts = seed)
//	created_at/updated -> PocketBase created/updated
func (s *coCreation) extendUsers() error {
	users, err := s.app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	users.Fields.Add(
		coText("display_name", 80, false),
		coSelect("account_status", false, "active", "suspended"),
		coSelect("registration_source", false, "invitation", "seed"),
	)
	if err := s.save(users); err != nil {
		return err
	}
	// Accounts created before the invitation-chain model act as the initialized
	// root members (D21 still pending). Existing accounts stay active.
	if _, err := s.app.DB().NewQuery("UPDATE users SET account_status = 'active' WHERE account_status = ''").Execute(); err != nil {
		return err
	}
	if _, err := s.app.DB().NewQuery("UPDATE users SET registration_source = 'seed' WHERE registration_source = ''").Execute(); err != nil {
		return err
	}
	s.users = users
	return nil
}

func (s *coCreation) createDictionaries() error {
	identities := coNew("collaboration_identities",
		coSelect("category", true, "main", "support"),
		coText("name", 80, true),
		coLongText("description", false),
		coBool("enabled"),
		coBool("xp_enabled"),
	)
	coIndexes(identities,
		"CREATE UNIQUE INDEX idx_collaboration_identities_category_name ON collaboration_identities (category, name)",
	)
	if err := s.save(identities); err != nil {
		return err
	}

	skills := coNew("skills",
		coText("name", 80, true),
		coText("category", 80, false),
		coLongText("description", false),
	)
	coIndexes(skills, "CREATE UNIQUE INDEX idx_skills_name ON skills (name)")
	if err := s.save(skills); err != nil {
		return err
	}

	s.identities = identities
	s.skills = skills
	return nil
}

func (s *coCreation) createProfileRelations() error {
	userIdentities := coNew("user_identities",
		coRelation("user_id", s.users.Id, true),
		coRelation("identity_id", s.identities.Id, true),
	)
	coIndexes(userIdentities,
		"CREATE UNIQUE INDEX idx_user_identities_user_identity ON user_identities (user_id, identity_id)",
		"CREATE INDEX idx_user_identities_identity ON user_identities (identity_id)",
	)
	if err := s.save(userIdentities); err != nil {
		return err
	}

	userSkills := coNew("user_skills",
		coRelation("user_id", s.users.Id, true),
		coRelation("skill_id", s.skills.Id, true),
		coText("proficiency", 20, false),
		coLongText("ability_note", false),
	)
	coIndexes(userSkills,
		"CREATE UNIQUE INDEX idx_user_skills_user_skill ON user_skills (user_id, skill_id)",
		"CREATE INDEX idx_user_skills_skill ON user_skills (skill_id)",
	)
	if err := s.save(userSkills); err != nil {
		return err
	}

	preferences := coNew("user_collaboration_preferences",
		coRelation("user_id", s.users.Id, true),
		coSelect("willingness", true, "open", "busy", "closed"),
		coJSON("help_types", false),
		coLongText("help_description", false),
		coNumber("weekly_hours", false, false, coFloat(0), coFloat(168)),
		coLongText("availability_note", false),
		coMultiSelect("expected_reward_types", "money", "product", "title", "credit", "learning", "none", "other"),
		coLongText("expected_reward_note", false),
		coBool("accept_invites"),
		coBool("receive_project_recommendations"),
		coSelect("visibility", true, "public", "members", "private"),
	)
	coIndexes(preferences,
		"CREATE UNIQUE INDEX idx_user_collaboration_preferences_user ON user_collaboration_preferences (user_id)",
	)
	return s.save(preferences)
}

func (s *coCreation) createMembershipInvitations() error {
	codes := coNew("membership_invitation_codes",
		coRelation("inviter_user_id", s.users.Id, true),
		coText("code_hash", 128, true),
		coSelect("status", true, "unused", "used", "revoked"),
		coRelation("invitee_user_id", s.users.Id, false),
		coDate("used_at", false),
		coDate("expires_at", false),
		coDate("revoked_at", false),
	)
	coIndexes(codes,
		"CREATE UNIQUE INDEX idx_membership_invitation_codes_hash ON membership_invitation_codes (code_hash)",
		// Unused codes keep invitee empty; a used code may only bind one invitee.
		"CREATE UNIQUE INDEX idx_membership_invitation_codes_invitee ON membership_invitation_codes (invitee_user_id) WHERE invitee_user_id != ''",
		"CREATE INDEX idx_membership_invitation_codes_inviter ON membership_invitation_codes (inviter_user_id, status)",
	)
	return s.save(codes)
}
