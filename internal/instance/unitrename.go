package instance

import (
	"fmt"
	"os"
)

// RenamePalaceUnitFile moves /etc/systemd/system/palman-<old>.service → palman-<new>.service.
// The caller must have stopped (and typically disabled) the old unit first.
func RenamePalaceUnitFile(oldName, newName string) error {
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	oldP := UnitPath(oldName)
	newP := UnitPath(newName)
	if _, err := os.Stat(oldP); err != nil {
		return fmt.Errorf("stat old unit: %w", err)
	}
	if _, err := os.Stat(newP); err == nil {
		return fmt.Errorf("refusing to overwrite existing unit file %s", newP)
	}
	return os.Rename(oldP, newP)
}
