package giDevice

import (
	"testing"
)

var backupSrv Backup

func setupBackupSrv(t *testing.T) {
	setupLockdownSrv(t)

	var err error
	if lockdownSrv, err = dev.lockdownService(); err != nil {
		t.Fatal(err)
	}

	if backupSrv, err = lockdownSrv.BackupService(); err != nil {
		t.Fatal(err)
	}
}

func Test_Backup(t *testing.T) {
	setupBackupSrv(t)

	err := backupSrv.StartBackup(dev.Properties().SerialNumber, "", map[string]interface{}{
		"ForceFullBackup": true,
	})
	if err != nil {
		t.Fatal(err)
	}
}
