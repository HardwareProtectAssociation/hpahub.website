//go:build !linux

package main

func maintenanceValidateRestore(archive, destination string) error {
	return errMaintenanceUnsupported
}
