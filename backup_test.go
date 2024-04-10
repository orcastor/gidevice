package giDevice

import (
	"fmt"
	"io"
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

type dummyRW struct {
}

func (*dummyRW) Write(p []byte) (n int, err error) {
	return
}

func (*dummyRW) Read(p []byte) (n int, err error) {
	return
}

func (*dummyRW) Close() error {
	return nil
}

type receiver struct {
}

func (recv *receiver) OnProgress(progress float64) {
	fmt.Printf("%.2f\r", progress)
}

func (recv *receiver) OnWriteFile(dpath, path string) (io.WriteCloser, error) {
	return &dummyRW{}, nil
}

func (recv *receiver) OnReadFile(dpath, path string) (io.ReadCloser, error) {
	return &dummyRW{}, nil
}

func (recv *receiver) OnAbort(err error) {
}

func (recv *receiver) OnFinish() {
}

func Test_Backup(t *testing.T) {
	setupBackupSrv(t)

	err := backupSrv.StartBackup(dev.Properties().SerialNumber, "", map[string]interface{}{
		"ForceFullBackup": true,
	})
	if err != nil {
		t.Fatal(err)
	}

	backupSrv.Backup(&receiver{})
}
