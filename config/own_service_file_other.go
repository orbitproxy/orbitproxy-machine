//go:build !darwin

package config

// OwnServiceFile is a no-op outside macOS.
// Linux keeps the root-owned file: the working sudo install path is unchanged.
// Windows services run as LocalSystem and do not use a Unix 0600 owner check.
func OwnServiceFile(string, *ServiceOwner) error {
	return nil
}

// OwnServiceTree is a no-op outside macOS.
func OwnServiceTree(string, *ServiceOwner) error {
	return nil
}
