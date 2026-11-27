package main

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const projectNumberRange = 90000

func randomProjectNumber() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(projectNumberRange))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("HPA-%05d", value.Int64()+10000), nil
}

func uniqueProjectNumber(app core.App) (string, error) {
	for attempt := 0; attempt < 128; attempt++ {
		number, err := randomProjectNumber()
		if err != nil {
			return "", err
		}
		_, err = app.FindFirstRecordByFilter("projects", "project_number = {:number}", dbx.Params{"number": number})
		if errors.Is(err, sql.ErrNoRows) {
			return number, nil
		}
		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("unable to allocate an unused project number")
}
