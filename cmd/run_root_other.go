//go:build !darwin

package main

func refuseRootForeground() error {
	return nil
}
