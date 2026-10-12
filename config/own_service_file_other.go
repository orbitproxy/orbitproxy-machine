//go:build !darwin && !linux

package config

// OwnServiceFile is a no-op on Windows.
// Windows services run as LocalSystem and do not use a Unix 0600 owner check.
func OwnServiceFile(string, *ServiceOwner) error {
	return nil
}

// OwnServiceTree is a no-op on Windows.
func OwnServiceTree(string, *ServiceOwner) error {
	return nil
}
