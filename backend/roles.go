package main

import "github.com/pocketbase/pocketbase/core"

func internalRole(record *core.Record) bool {
	role := siteRole(record)
	return role == "member" || role == "admin" || role == "octopus"
}
