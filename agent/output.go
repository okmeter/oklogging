package agent

import (
	"net"
	"time"
	"encoding/json"
	"encoding/binary"
	"fmt"
	"log"
)

type Output interface {
	Close()
	String() string
	Write([]byte) error
}

type BlackHoleOutput struct {}

func (o *BlackHoleOutput) Close() {
	return
}

func (o *BlackHoleOutput) String() string {
	return "BlackHole()"
}

func (o *BlackHoleOutput) Write([]byte) error {
	return nil
}

func send(conn net.Conn, payload []byte, timeout time.Duration) error {
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
	}
	if err := binary.Write(conn, binary.LittleEndian, int32(len(payload))); err != nil {
		return err
	}
	if _, err := conn.Write(payload); err != nil {
		return err
	}
	var status int32
	if err := binary.Read(conn, binary.LittleEndian, &status); err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("got %d response from server:", status)
	}
	return nil
}

type TcpOutput struct {
	server string
	conn net.Conn
	labels map[string]string
	timeout time.Duration
}

func NewTcpOutput(server string, labels map[string]string, timeout time.Duration) *TcpOutput {
	return &TcpOutput{
		server: server,
		timeout: timeout,
		labels: labels,
	}
}

func (o *TcpOutput) String() string {
	return fmt.Sprintf("TcpOutput(%s, %v)", o.server, o.labels)
}

func (o *TcpOutput) Close() {
	if o.conn != nil {
		o.disconnect()
		return
	}
	return
}

func (o *TcpOutput) Write(data []byte) error {
	if o.conn == nil {
		if err := o.connect(); err != nil {
			return err
		}
	}
	start := time.Now()
	if err := send(o.conn, data, o.timeout); err != nil {
		o.disconnect()
		return err
	}
	writeHistogram.Observe(time.Since(start).Seconds())
	bytesWritten.Add(float64(len(data)))
	return nil
}

func (o *TcpOutput) disconnect() error {
	err := o.conn.Close()
	o.conn = nil
	return err
}

func (o *TcpOutput) connect() error {
	labelsJson, err := json.Marshal(o.labels)
	if err != nil {
		return err
	}
	o.conn, err = net.DialTimeout("tcp", o.server, o.timeout)
	if err != nil {
		return err
	}
	if err := send(o.conn, labelsJson, o.timeout); err != nil {
		o.disconnect()
		return err
	}
	log.Println(o.String(), "connected")
	return nil
}
