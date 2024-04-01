package libimobiledevice

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const BackupServiceName = "com.apple.mobilebackup2"

func NewBackupClient(innerConn InnerConn) *BackupClient {
	return &BackupClient{
		client: newServicePacketClient(innerConn),
	}
}

type BackupClient struct {
	client *servicePacketClient
}

func (c *BackupClient) NewBinaryPacket(req interface{}) (Packet, error) {
	return c.client.NewBinaryPacket(req)
}

func (c *BackupClient) SendPacket(pkt Packet) (err error) {
	return c.client.SendPacket(pkt)
}

func (c *BackupClient) ReceivePacket() (respPkt Packet, err error) {
	var bufLen []byte
	if bufLen, err = c.client.innerConn.Read(4); err != nil {
		return nil, fmt.Errorf("lockdown(Backup) receive: %w", err)
	}
	lenPkg := binary.BigEndian.Uint32(bufLen)

	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(bufLen)

	var buf []byte
	if buf, err = c.client.innerConn.Read(int(lenPkg)); err != nil {
		return nil, fmt.Errorf("lockdown(Backup) receive: %w", err)
	}
	buffer.Write(buf)

	if respPkt, err = new(servicePacket).Unpack(buffer); err != nil {
		return nil, fmt.Errorf("lockdown(Backup) receive: %w", err)
	}

	debugLog(fmt.Sprintf("<-- %s\n", respPkt))

	return
}

func (c *BackupClient) SendRaw(raw []byte) (err error) {
	return c.client.innerConn.Write(raw)
}

func (c *BackupClient) ReadRaw(length int) (buf []byte, err error) {
	return c.client.innerConn.Read(length)
}
