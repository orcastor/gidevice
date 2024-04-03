package giDevice

import (
	"encoding/binary"
	"fmt"
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

func recvFileName() string {
	var l int
	for {
		length, _ := backupSrv.ReadRaw(4)
		if len(length) == 0 {
			continue
		}
		l = int(binary.BigEndian.Uint32(length))
		if l <= 0 {
			return ""
		}
		break
	}
	name, _ := backupSrv.ReadRaw(l)
	return string(name)
}

func recvFileLength() uint32 {
	length, _ := backupSrv.ReadRaw(4)
	if len(length) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(length)
}

const CODE_SUCCESS = 0x00
const CODE_ERROR_LOCAL = 0x06
const CODE_ERROR_REMOTE = 0x0b
const CODE_FILE_DATA = 0x0c

const BLOCK_SIZE = 4194304

func Test_Backup(t *testing.T) {
	setupBackupSrv(t)

	err := backupSrv.StartBackup(dev.Properties().SerialNumber, "", map[string]interface{}{
		"ForceFullBackup": true,
	})
	if err != nil {
		t.Fatal(err)
	}

	progress := 0.0

	for {
		resp, err := backupSrv.ReceivePacket()
		if err != nil {
			t.Fatal(err)
			break
		}
		switch resp[0].(string) {
		case "DLMessageDownloadFiles":
			progress = resp[3].(float64)

			var raw [4]byte

			errString := "No such file or directory"

			errs := map[string]interface{}{}
			fmt.Println(resp[0].(string))

			for _, path := range resp[1].([]interface{}) {
				fmt.Println(path)
				// path
				binary.BigEndian.PutUint32(raw[:], uint32(len(path.(string))))
				_ = backupSrv.SendRaw(raw[:])
				_ = backupSrv.SendRaw([]byte(path.(string)))

				/*
					var record struct {
						SnapshotState string    `plist:"SnapshotState"`
						IsFullBackup  bool      `plist:"IsFullBackup"`
						UUID          string    `plist:"UUID"`
						BackupState   string    `plist:"BackupState"`
						Date          time.Time `plist:"Date"`
						Version       string    `plist:"Version"`
					}
					record.UUID = dev.Properties().UDID
					record.SnapshotState = "uploading"
					record.IsFullBackup = true
					record.BackupState = "new"
					record.Version = "3.3"
					record.Date = time.Now()

					bs, _ := plist.Marshal(record, plist.BinaryFormat)
					fmt.Println(string(bs))

					binary.BigEndian.PutUint32(raw[:], uint32(len(bs)))
					_ = backupSrv.SendRaw(append(raw[:], byte(CODE_FILE_DATA)))
					_ = backupSrv.SendRaw(bs)
				*/

				// binary.BigEndian.PutUint32(raw[:], 0)
				// _ = backupSrv.SendRaw(append(raw[:], byte(CODE_FILE_DATA)))

				// binary.BigEndian.PutUint32(raw, fileLen)
				// _ = backupSrv.SendRaw(append(raw, byte(CODE_FILE_DATA)))
				// _ = backupSrv.SendRaw(filedata)

				binary.BigEndian.PutUint32(raw[:], uint32(len(errString)+1))
				_ = backupSrv.SendRaw(append(append(raw[:], byte(CODE_ERROR_LOCAL)), []byte(errString)...))

				errs[path.(string)] = map[string]interface{}{
					"DLFileErrorString": errString,
					"DLFileErrorCode":   -6,
				}
			}

			/*
							<?xml version="1.0" encoding="UTF-8"?>
				<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
				<plist version="1.0">
				<array>
				        <string>DLMessageStatusResponse</string>
				        <integer>18446744073709551603</integer>
				        <string>Multi status</string>
				        <dict>
				                <key>d2e6c832cd7d0a4cff535b59e4f567bbfef65dc2/Status.plist</key>
				                <dict>
				                        <key>DLFileErrorString</key>
				                        <string>No such file or directory</string>
				                        <key>DLFileErrorCode</key>
				                        <integer>18446744073709551610</integer>
				                </dict>
				        </dict>
				</array>
				</plist>
			*/

			/* send terminating 0 dword */
			binary.BigEndian.PutUint32(raw[:], 0)
			_ = backupSrv.SendRaw(raw[:])

			if len(errs) > 0 {
				backupSrv.SendPacket([]interface{}{
					"DLMessageStatusResponse",
					-13,
					"Multi status",
					errs,
				})
			} else {
				backupSrv.SendPacket([]interface{}{
					"DLMessageStatusResponse",
					0,
					"___EmptyParameterString___",
					map[string]interface{}{},
				})
			}
		case "DLMessageUploadFiles":
			progress = resp[2].(float64)

			var fileLen uint32
			for {
				dname := recvFileName()
				if dname == "" {
					break
				}
				fname := recvFileName()
				if fname == "" {
					break
				}

				// fmt.Printf("DeviceFile:%s\n", dname)
				// fmt.Printf("File:%s\n", fname)

				length, _ := backupSrv.ReadRaw(4)
				if len(length) != 4 {
					break
				}
				fileLen = binary.BigEndian.Uint32(length)

				code, _ := backupSrv.ReadRaw(1)
				if len(code) != 1 {
					break
				}

				lastCode := code

				for code[0] == CODE_FILE_DATA {
					if int(fileLen)-1 > 0 {
						fileLen = fileLen - 1
						for fileLen != 0 {
							if fileLen > BLOCK_SIZE {
								_, _ = backupSrv.ReadRaw(BLOCK_SIZE)
								fileLen -= BLOCK_SIZE
							} else {
								_, _ = backupSrv.ReadRaw(int(fileLen))
								fileLen -= fileLen
							}
						}
					}

					fileLen = 0
					length, _ := backupSrv.ReadRaw(4)
					if len(length) != 4 {
						break
					}
					fileLen = binary.BigEndian.Uint32(length)

					if fileLen > 0 {
						lastCode = code
						code, _ = backupSrv.ReadRaw(1)
					} else {
						break
					}
				}

				if fileLen <= 0 {
					break
				}

				if code[0] == CODE_ERROR_REMOTE {
					msg, _ := backupSrv.ReadRaw(int(fileLen) - 1)
					if lastCode[0] != CODE_FILE_DATA {
						fmt.Println(string(msg))
					}
				}
			}

			if int(fileLen)-1 > 0 {
				_, _ = backupSrv.ReadRaw(int(fileLen) - 1)
			}

			backupSrv.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				map[string]interface{}{},
			})
		case "DLMessageGetFreeDiskSpace":
			backupSrv.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				uint64(8000000000000),
			})
		case "DLMessagePurgeDiskSpace":
			backupSrv.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				-1,
				"Operation not supported",
				map[string]interface{}{},
			})
		case "DLMessageCreateDirectory":
			backupSrv.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				"___EmptyParameterString___",
			})
		case "DLMessageCopyItem":
			backupSrv.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				map[string]interface{}{},
			})
		case "DLMessageMoveFiles", "DLMessageMoveItems", "DLMessageRemoveFiles", "DLMessageRemoveItems":
			progress = resp[3].(float64)
			backupSrv.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				map[string]interface{}{},
			})
		case "DLMessageDisconnect":
		case "DLMessageProcessMessage":
			checkErr := resp[1].(map[string]interface{})
			if checkErr["MessageName"].(string) != "Response" {
				fmt.Println("`MessageName` not equal")
			}

			if checkErr["ErrorCode"].(uint64) != 0 {
				t.Fatal(fmt.Errorf("received an error(%d): %s\n", checkErr["ErrorCode"].(uint64), checkErr["ErrorDescription"].(string)))
			}
		}

		fmt.Printf("%.2f %%\r", progress)
		if progress >= 100.00 {
			break
		}
	}

	fmt.Println("Finished")
}
