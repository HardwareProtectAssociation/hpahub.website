package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// coCreation builds the project co-creation platform schema described in
// 项目共创平台_AI开发说明.md §8–§9. Collection and field names follow that
// document; the PocketBase adaptations (built-in ids/timestamps, relation
// storage, integer minor units for money, ...) are documented in
// backend/migrations/README.md.
type coCreation struct {
	app core.App

	users         *core.Collection
	identities    *core.Collection
	skills        *core.Collection
	projects      *core.Collection
	stages        *core.Collection
	needs         *core.Collection
	rewards       *core.Collection
	sponsorships  *core.Collection
	contents      *core.Collection
	comments      *core.Collection
	agreements    *core.Collection
	tasks         *core.Collection
	roles         *core.Collection
	contributions *core.Collection
	ballots       *core.Collection
	thresholds    *core.Collection
}

func buildCoCreationSchema(app core.App) error {
	s := &coCreation{app: app}
	steps := []func() error{
		s.extendUsers,
		s.createDictionaries,
		s.createProfileRelations,
		s.createMembershipInvitations,
		s.createProjects,
		s.createStages,
		s.createMembership,
		s.createSponsorshipDefinitions,
		s.createSponsorships,
		s.createContents,
		s.createMediaAssets,
		s.linkProjectAssets,
		s.createShowcases,
		s.createInteractions,
		s.createAgreements,
		s.createUnions,
		s.createSkillNeeds,
		s.createGovernanceQueues,
		s.createPlatformRecords,
		s.createTasks,
		s.createParticipantRoles,
		s.createTaskContributions,
		s.createXpVoting,
		s.createCareerLedger,
		s.createCompletionRatings,
		s.seedDictionaries,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (s *coCreation) save(collection *core.Collection) error {
	return s.app.Save(collection)
}

// coNew starts a base collection with the given fields plus the created/updated
// autodate fields that map the document's created_at/updated_at columns.
func coNew(name string, fields ...core.Field) *core.Collection {
	collection := core.NewBaseCollection(name)
	collection.Fields.Add(fields...)
	collection.Fields.Add(
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)
	return collection
}

func coIndexes(collection *core.Collection, indexes ...string) {
	collection.Indexes = append(collection.Indexes, indexes...)
}

func coText(name string, max int, required bool) *core.TextField {
	return &core.TextField{Name: name, Max: max, Required: required}
}

func coLongText(name string, required bool) *core.TextField {
	return coText(name, 20000, required)
}

func coSelect(name string, required bool, values ...string) *core.SelectField {
	return &core.SelectField{Name: name, Values: values, MaxSelect: 1, Required: required}
}

func coMultiSelect(name string, values ...string) *core.SelectField {
	return &core.SelectField{Name: name, Values: values, MaxSelect: len(values)}
}

func coRelation(name, collectionID string, required bool) *core.RelationField {
	return &core.RelationField{Name: name, CollectionId: collectionID, MaxSelect: 1, Required: required}
}

func coDate(name string, required bool) *core.DateField {
	return &core.DateField{Name: name, Required: required}
}

func coNumber(name string, required, onlyInt bool, min, max *float64) *core.NumberField {
	return &core.NumberField{Name: name, Required: required, OnlyInt: onlyInt, Min: min, Max: max}
}

func coBool(name string) *core.BoolField {
	return &core.BoolField{Name: name}
}

func coJSON(name string, required bool) *core.JSONField {
	return &core.JSONField{Name: name, Required: required}
}

func coFloat(value float64) *float64 {
	return &value
}
