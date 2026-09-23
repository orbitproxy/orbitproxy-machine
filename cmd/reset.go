package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/runlock"
	"github.com/spf13/cobra"
)

const binDirName = "bin"

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Remove local OrbitProxy configuration",
	Long:  "Stop the client and OS service, then delete machine configuration under the data root. The program in <root>/bin is kept.",
	Args:  cobra.NoArgs,
	RunE:  runReset,
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove OrbitProxy from this machine",
	Long:  "Stop the client and OS service, then delete the local data root including ~/.orbitproxy/bin. This cannot be undone. Reinstall from the console to use OrbitProxy again.",
	Args:  cobra.NoArgs,
	RunE:  runDelete,
}

func init() {
	rootCmd.AddCommand(resetCmd, deleteCmd)
}

type resetTarget struct {
	path            string
	mustBeOrbitRoot bool
}

type resetDeps struct {
	serviceInstalled func() (bool, error)
	requireAdmin     func() error
	removeService    func() error
}

func defaultResetDeps() resetDeps {
	return resetDeps{
		serviceInstalled: platformServiceInstalled,
		requireAdmin:     requireAdmin,
		removeService:    removePlatformService,
	}
}

func runReset(cmd *cobra.Command, _ []string) error {
	return runPurge(cmd, false)
}

func runDelete(cmd *cobra.Command, _ []string) error {
	return runPurge(cmd, true)
}

func runPurge(cmd *cobra.Command, removeProgram bool) error {
	targets, home, err := resolveResetTargets(workDirFlag, removeProgram)
	if err != nil {
		return err
	}
	return performPurge(cmd.OutOrStdout(), defaultResetDeps(), targets, home, removeProgram)
}

func resolveResetTargets(explicit string, removeProgram bool) ([]resetTarget, string, error) {
	owner, err := config.ResolveServiceOwner()
	if err != nil {
		return nil, "", err
	}
	dataRoot, err := config.ResolveServiceRoot(explicit, owner)
	if err != nil {
		return nil, "", err
	}
	installRoot, err := config.ResolveServiceRoot("", owner)
	if err != nil {
		return nil, "", err
	}

	targets := []resetTarget{{
		path:            dataRoot,
		mustBeOrbitRoot: strings.TrimSpace(explicit) == "",
	}}
	if removeProgram && filepath.Clean(installRoot) != filepath.Clean(dataRoot) {
		targets = append(targets, resetTarget{path: installRoot, mustBeOrbitRoot: true})
	}
	return targets, owner.Home, nil
}

func performPurge(out io.Writer, deps resetDeps, targets []resetTarget, home string, removeProgram bool) error {
	if deps.serviceInstalled == nil || deps.requireAdmin == nil || deps.removeService == nil {
		return fmt.Errorf("purge dependencies are not configured")
	}

	installed, err := deps.serviceInstalled()
	if err != nil {
		return err
	}
	if installed {
		if err := deps.requireAdmin(); err != nil {
			return purgeNeedsAdminError(err, removeProgram)
		}
		if err := deps.removeService(); err != nil {
			return err
		}
		fmt.Fprintln(out, "Removed OS service")
	}

	for _, target := range targets {
		if err := wipeAllowed(target.path, target.mustBeOrbitRoot, home); err != nil {
			return err
		}
		if err := stopLocalProcesses(out, target.path); err != nil {
			return err
		}
	}

	removed := 0
	for _, target := range targets {
		var (
			existed bool
			err     error
		)
		if removeProgram {
			existed, err = removeTree(target.path)
			if err != nil {
				return err
			}
			if existed {
				fmt.Fprintf(out, "Deleted %s\n", target.path)
			}
		} else {
			existed, err = removeConfig(out, target.path)
			if err != nil {
				return err
			}
		}
		if existed {
			removed++
		}
	}
	if !installed && removed == 0 {
		fmt.Fprintln(out, "Nothing to remove")
		return nil
	}
	if removeProgram {
		fmt.Fprintln(out, "OrbitProxy is removed from this machine. Reinstall from the console to use it again.")
		return nil
	}
	fmt.Fprintln(out, "OrbitProxy configuration removed. The program was kept.")
	return nil
}

func purgeNeedsAdminError(base error, removeProgram bool) error {
	command := "reset"
	if removeProgram {
		command = "delete"
	}
	exe, exeErr := executablePath()
	if runtime.GOOS == "windows" {
		if exeErr != nil {
			return fmt.Errorf("%w\nOrbitProxy is installed as a system service. Re-run %s from an elevated prompt", base, command)
		}
		return fmt.Errorf("%w\nOrbitProxy is installed as a system service. Re-run %s from an elevated prompt:\n\n  %s %s\n", base, command, exe, command)
	}
	if exeErr != nil {
		return fmt.Errorf("%w\nOrbitProxy is installed as a system service. Re-run %s with sudo", base, command)
	}
	return fmt.Errorf("%w\nOrbitProxy is installed as a system service. Re-run:\n\n  sudo %s %s\n", base, exe, command)
}

func wipeAllowed(path string, mustBeOrbitRoot bool, home string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("refusing to delete an empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path %s: %w", path, err)
	}
	clean := filepath.Clean(abs)
	volumeRoot := filepath.VolumeName(clean) + string(os.PathSeparator)
	if clean == "/" || clean == volumeRoot || clean == filepath.VolumeName(clean)+"/" {
		return fmt.Errorf("refusing to delete %s", clean)
	}
	if home != "" && filepath.Clean(home) == clean {
		return fmt.Errorf("refusing to delete home directory %s", clean)
	}
	if mustBeOrbitRoot && filepath.Base(clean) != ".orbitproxy" {
		return fmt.Errorf("refusing to delete %s (not an OrbitProxy data root)", clean)
	}
	return nil
}

func stopLocalProcesses(out io.Writer, root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("list %s: %w", root, err)
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(root, ent.Name())
		if err := stopLockHolder(out, dir); err != nil {
			return err
		}
		if err := stopExecPIDs(out, dir); err != nil {
			return err
		}
	}
	return nil
}

func stopLockHolder(out io.Writer, machineDir string) error {
	holder, err := runlock.Inspect(machineDir)
	if err != nil {
		return err
	}
	if holder == nil || holder.PID == os.Getpid() {
		return nil
	}
	if err := terminatePID(holder.PID); err != nil {
		return fmt.Errorf("stop pid %d: %w", holder.PID, err)
	}
	fmt.Fprintf(out, "Stopped process pid %d\n", holder.PID)
	return nil
}

func stopExecPIDs(out io.Writer, machineDir string) error {
	data, err := os.ReadFile(filepath.Join(machineDir, "exec_pids.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read exec pid file: %w", err)
	}
	var records []struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil
	}
	self := os.Getpid()
	for _, rec := range records {
		if rec.PID <= 0 || rec.PID == self {
			continue
		}
		if err := terminatePID(rec.PID); err != nil {
			return fmt.Errorf("stop exec pid %d: %w", rec.PID, err)
		}
		fmt.Fprintf(out, "Stopped process pid %d\n", rec.PID)
	}
	return nil
}

func removeConfig(out io.Writer, root string) (bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("list %s: %w", root, err)
	}
	removed := false
	for _, ent := range entries {
		if ent.Name() == binDirName {
			continue
		}
		path := filepath.Join(root, ent.Name())
		if err := os.RemoveAll(path); err != nil {
			return false, fmt.Errorf("delete %s: %w", path, err)
		}
		fmt.Fprintf(out, "Deleted %s\n", path)
		removed = true
	}
	return removed, nil
}

func removeTree(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.RemoveAll(path); err != nil {
		return false, fmt.Errorf("delete %s: %w", path, err)
	}
	return true, nil
}
