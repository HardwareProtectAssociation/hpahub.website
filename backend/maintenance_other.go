//go:build !linux

package main

func maintenanceSnapshot(gpu, warehouse, auth, output string) error {
	return errMaintenanceUnsupported
}

func maintenanceValidateRestore(archive, destination string) error {
	return errMaintenanceUnsupported
}
