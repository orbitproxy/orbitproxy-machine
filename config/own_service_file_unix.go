//go:build darwin || linux

package config

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// OwnServiceFile makes path readable by the service user.
// sudo install runs as root and writes 0600 files; launchd and systemd run as the installing user.
func OwnServiceFile(path string, owner *ServiceOwner) error {
	uid, gid, ok, err := serviceOwnerIDs(owner)
	if err != nil || !ok {
		return err
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %s to %s: %w", path, owner.Username, err)
	}
	return nil
}

// OwnServiceTree gives root-owned files under root back to the service user.
// Mode bits stay as they are. Files already owned by that user are left alone.
func OwnServiceTree(root string, owner *ServiceOwner) error {
	uid, gid, ok, err := serviceOwnerIDs(owner)
	if err != nil || !ok {
		return err
	}
	return filepath.WalkDir(root, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		ownedByRoot, err := fileOwnedByRoot(path)
		if err != nil {
			return err
		}
		if !ownedByRoot {
			return nil
		}
		if err := os.Lchown(path, uid, gid); err != nil {
			return fmt.Errorf("chown %s to %s: %w", path, owner.Username, err)
		}
		return nil
	})
}

func serviceOwnerIDs(owner *ServiceOwner) (uid int, gid int, ok bool, err error) {
	if owner == nil {
		return 0, 0, false, nil
	}
	name := strings.TrimSpace(owner.Username)
	if name == "" || name == "root" {
		return 0, 0, false, nil
	}
	account, err := user.Lookup(name)
	if err != nil {
		return 0, 0, false, fmt.Errorf("lookup service user %s: %w", name, err)
	}
	uid, err = strconv.Atoi(account.Uid)
	if err != nil {
		return 0, 0, false, fmt.Errorf("parse uid %s: %w", account.Uid, err)
	}
	gid = -1
	if account.Gid != "" {
		gid, err = strconv.Atoi(account.Gid)
		if err != nil {
			return 0, 0, false, fmt.Errorf("parse gid %s: %w", account.Gid, err)
		}
	}
	return uid, gid, true, nil
}

func fileOwnedByRoot(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return false, nil
	}
	return stat.Uid == 0, nil
}
