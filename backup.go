package giDevice

import (
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

func (s *backup) StartBackup(tid, sid string, options map[string]interface{}) (err error) {
	if err = s.exchange(); err != nil {
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

	fmt.Println(req)

	return s.SendPacket(req)
}

func (s *backup) SendPacket(req []interface{}) (err error) {
	if err = s.exchange(); err != nil {
		return err
	}
	var pkt libimobiledevice.Packet
	if pkt, err = s.client.NewBinaryPacket(req); err != nil {
		return err
	}

	return s.client.SendPacket(pkt)
}

func (s *backup) ReceivePacket() (resp []interface{}, err error) {
	if err = s.exchange(); err != nil {
		return nil, err
	}

	var respPkt libimobiledevice.Packet
	if respPkt, err = s.client.ReceivePacket(); err != nil {
		return resp, err
	}

	err = respPkt.Unmarshal(&resp)
	return resp, err
}

func (s *backup) SendRaw(raw []byte) (err error) {
	if err = s.exchange(); err != nil {
		return err
	}

	return s.client.SendRaw(raw)
}

func (s *backup) ReadRaw(length int) (buf []byte, err error) {
	if err = s.exchange(); err != nil {
		return nil, err
	}

	return s.client.ReadRaw(length)
}

func (s *backup) exchange() (err error) {
	if s.exchanged {
		return
	}

	var respPkt libimobiledevice.Packet
	if respPkt, err = s.client.ReceivePacket(); err != nil {
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
	if pkt, err = s.client.NewBinaryPacket(req); err != nil {
		return err
	}

	if err = s.client.SendPacket(pkt); err != nil {
		return err
	}

	if respPkt, err = s.client.ReceivePacket(); err != nil {
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

	if pkt, err = s.client.NewBinaryPacket(req); err != nil {
		return err
	}

	if err = s.client.SendPacket(pkt); err != nil {
		return err
	}

	if respPkt, err = s.client.ReceivePacket(); err != nil {
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

	s.exchanged = true
	return
}
