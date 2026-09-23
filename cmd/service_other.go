//go:build !linux && !darwin && !windows

package main

func requireAdmin() error {
	return errServiceUnsupported
}

func installPlatformService(serviceInstallArgs) error {
	return errServiceUnsupported
}

func platformServiceInstalled() (bool, error) {
	return false, nil
}

func removePlatformService() error {
	return nil
}

func uninstallPlatformService() error {
	return errServiceUnsupported
}

func startPlatformService() error {
	return errServiceUnsupported
}

func stopPlatformService() error {
	return errServiceUnsupported
}

func restartPlatformService() error {
	return errServiceUnsupported
}

func statusPlatformService() (string, error) {
	return "", errServiceUnsupported
}
