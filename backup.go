package giDevice

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/electricbubble/gidevice/pkg/libimobiledevice"
)

var _ Backup = (*backup)(nil)

func newBackup(client *libimobiledevice.BackupClient) *backup {
	return &backup{
		client:    client,
		exchanged: false,
	}
}

type backup struct {
	client    *libimobiledevice.BackupClient
	exchanged bool
}

func (b *backup) StartBackup(tid, sid string, options map[string]interface{}) (err error) {
	if err = b.exchange(); err != nil {
		return err
	}

	m := map[string]interface{}{
		"MessageName":      "Backup",
		"TargetIdentifier": tid,
	}

	if options != nil {
		m["Options"] = options
	}

	if sid != "" {
		m["SourceIdentifier"] = sid
	}

	// link service
	req := []interface{}{
		"DLMessageProcessMessage",
		m,
	}

	return b.SendPacket(req)
}

func (b *backup) recvFileName() string {
	var l int
	for {
		length, _ := b.ReadRaw(4)
		if len(length) == 0 {
			continue
		}
		l = int(binary.BigEndian.Uint32(length))
		if l <= 0 {
			return ""
		}
		break
	}
	name, _ := b.ReadRaw(l)
	return string(name)
}

func (b *backup) recvFileLength() uint32 {
	length, _ := b.ReadRaw(4)
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

func (b *backup) Backup() {
	progress := 0.0

	for {
		resp, err := b.ReceivePacket()
		if err != nil {
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
				_ = b.SendRaw(raw[:])
				_ = b.SendRaw([]byte(path.(string)))

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
					_ = b.SendRaw(append(raw[:], byte(CODE_FILE_DATA)))
					_ = b.SendRaw(bs)
				*/

				// binary.BigEndian.PutUint32(raw[:], 0)
				// _ = b.SendRaw(append(raw[:], byte(CODE_FILE_DATA)))

				// binary.BigEndian.PutUint32(raw, fileLen)
				// _ = b.SendRaw(append(raw, byte(CODE_FILE_DATA)))
				// _ = b.SendRaw(filedata)

				binary.BigEndian.PutUint32(raw[:], uint32(len(errString)+1))
				_ = b.SendRaw(append(append(raw[:], byte(CODE_ERROR_LOCAL)), []byte(errString)...))

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
			_ = b.SendRaw(raw[:])

			if len(errs) > 0 {
				b.SendPacket([]interface{}{
					"DLMessageStatusResponse",
					-13,
					"Multi status",
					errs,
				})
			} else {
				b.SendPacket([]interface{}{
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
				dname := b.recvFileName()
				if dname == "" {
					break
				}
				fname := b.recvFileName()
				if fname == "" {
					break
				}

				// fmt.Printf("DeviceFile:%s\n", dname)
				fmt.Printf("%.2f %% File:%s\r", progress, fname)

				length, _ := b.ReadRaw(4)
				if len(length) != 4 {
					break
				}
				fileLen = binary.BigEndian.Uint32(length)

				code, _ := b.ReadRaw(1)
				if len(code) != 1 {
					break
				}

				lastCode := code

				for code[0] == CODE_FILE_DATA {
					if int(fileLen)-1 > 0 {
						fileLen = fileLen - 1
						for fileLen != 0 {
							if fileLen > BLOCK_SIZE {
								_, _ = b.ReadRaw(BLOCK_SIZE)
								fileLen -= BLOCK_SIZE
							} else {
								_, _ = b.ReadRaw(int(fileLen))
								fileLen -= fileLen
							}
						}
					}

					fileLen = 0
					length, _ := b.ReadRaw(4)
					if len(length) != 4 {
						break
					}
					fileLen = binary.BigEndian.Uint32(length)

					if fileLen > 0 {
						lastCode = code
						code, _ = b.ReadRaw(1)
					} else {
						break
					}
				}

				if fileLen <= 0 {
					break
				}

				if code[0] == CODE_ERROR_REMOTE {
					msg, _ := b.ReadRaw(int(fileLen) - 1)
					if lastCode[0] != CODE_FILE_DATA {
						fmt.Println(string(msg))
					}
				}
			}

			if int(fileLen)-1 > 0 {
				_, _ = b.ReadRaw(int(fileLen) - 1)
			}

			b.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				map[string]interface{}{},
			})
		case "DLMessageGetFreeDiskSpace":
			b.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				uint64(8000000000000),
			})
		case "DLMessagePurgeDiskSpace":
			b.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				-1,
				"Operation not supported",
				map[string]interface{}{},
			})
		case "DLMessageCreateDirectory":
			b.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				"___EmptyParameterString___",
			})
		case "DLMessageCopyItem":
			b.SendPacket([]interface{}{
				"DLMessageStatusResponse",
				0,
				"___EmptyParameterString___",
				map[string]interface{}{},
			})
		case "DLMessageMoveFiles", "DLMessageMoveItems", "DLMessageRemoveFiles", "DLMessageRemoveItems":
			progress = resp[3].(float64)
			b.SendPacket([]interface{}{
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
				fmt.Printf("received an error(%d): %s\n", checkErr["ErrorCode"].(uint64), checkErr["ErrorDescription"].(string))
			}

			if c, ok := checkErr["Content"]; ok {
				fmt.Printf("Content: %s\n", c.(string))
			}
		}

		fmt.Printf("%.2f %%\r", progress)
		if progress >= 100.00 {
			break
		}
	}
	fmt.Println("Finished")
}

func (b *backup) SendPacket(req []interface{}) (err error) {
	if err = b.exchange(); err != nil {
		return err
	}
	var pkt libimobiledevice.Packet
	if pkt, err = b.client.NewBinaryPacket(req); err != nil {
		return err
	}

	return b.client.SendPacket(pkt)
}

func (b *backup) ReceivePacket() (resp []interface{}, err error) {
	if err = b.exchange(); err != nil {
		return nil, err
	}

	var respPkt libimobiledevice.Packet
	if respPkt, err = b.client.ReceivePacket(); err != nil {
		return resp, err
	}

	err = respPkt.Unmarshal(&resp)
	return resp, err
}

func (b *backup) SendRaw(raw []byte) (err error) {
	if err = b.exchange(); err != nil {
		return err
	}

	return b.client.SendRaw(raw)
}

func (b *backup) ReadRaw(length int) (buf []byte, err error) {
	if err = b.exchange(); err != nil {
		return nil, err
	}

	return b.client.ReadRaw(length)
}

func (b *backup) exchange() (err error) {
	if b.exchanged {
		return
	}

	var respPkt libimobiledevice.Packet
	if respPkt, err = b.client.ReceivePacket(); err != nil {
		return err
	}

	var resp []interface{}
	if err = respPkt.Unmarshal(&resp); err != nil {
		return err
	}

	req := []interface{}{
		"DLMessageVersionExchange",
		"DLVersionsOk",
		resp[1],
	}

	var pkt libimobiledevice.Packet
	if pkt, err = b.client.NewBinaryPacket(req); err != nil {
		return err
	}

	if err = b.client.SendPacket(pkt); err != nil {
		return err
	}

	if respPkt, err = b.client.ReceivePacket(); err != nil {
		return err
	}

	if err = respPkt.Unmarshal(&resp); err != nil {
		return err
	}

	if resp[3].(string) != "DLMessageDeviceReady" {
		return fmt.Errorf("message device not ready %s", resp[3])
	}

	// link service
	req = []interface{}{
		"DLMessageProcessMessage",
		map[string]interface{}{
			"MessageName":               "Hello",
			"SupportedProtocolVersions": []float64{2.0, 2.1},
		},
	}

	if pkt, err = b.client.NewBinaryPacket(req); err != nil {
		return err
	}

	if err = b.client.SendPacket(pkt); err != nil {
		return err
	}

	if respPkt, err = b.client.ReceivePacket(); err != nil {
		return err
	}

	if err = respPkt.Unmarshal(&resp); err != nil {
		return err
	}

	fmt.Println(resp)

	if resp[4].(string) != "DLMessageProcessMessage" {
		return fmt.Errorf("message device not ready %s", fmt.Sprint(resp[4]))
	}

	backup := resp[5].(map[string]interface{})
	if backup["MessageName"].(string) != "Response" {
		return errors.New("`MessageName` not equal")
	}

	if backup["ErrorCode"].(uint64) != 0 {
		return fmt.Errorf("received an error(%d): %s", backup["ErrorCode"].(uint64), backup["ErrorDescription"].(string))
	}

	// [ErrorCode:208 ErrorDescription:Device locked (MBErrorDomain/208) MessageName:Response

	b.exchanged = true
	return
}
