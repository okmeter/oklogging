package main

import (
	"log"
	"flag"
	"encoding/json"
	"os"
	"path"
	"net"
	"encoding/binary"
	"io"
	"time"
)

const (
	timeout = 10 * time.Second
)

type Msg struct {
	size int
	payload []byte
}

func (msg *Msg) Bytes() []byte {
	return msg.payload[:msg.size]
}

func (msg *Msg) Len() int {
	return msg.size
}

func readMsg(conn net.Conn, msg *Msg, timeout time.Duration) error {
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
	}
	var size int32
	if err := binary.Read(conn, binary.LittleEndian, &size); err != nil {
		return err
	}
	msg.size = int(size)
	if len(msg.payload) < msg.size {
		msg.payload = make([]byte, msg.size)
	}
	_, err := io.ReadAtLeast(conn, msg.payload, msg.size)
	if err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return err
	}
	return nil
}

func sendResponse(conn net.Conn, status int32, timeout time.Duration) error {
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
	}
	if err := binary.Write(conn, binary.LittleEndian, &status); err != nil {
		return err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return err
	}
	return nil
}

func listenAndServe(listen string, logDir string) (error) {
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		go handleConnection(c, logDir)
	}
}

func handleConnection(conn net.Conn, logDir string) {
	defer conn.Close()
	msg := &Msg{}
	if err := readMsg(conn, msg, timeout); err != nil {
		log.Println("failed to read msg from", conn.RemoteAddr(), err)
		return
	}
	labels := map[string]string{}
	if err := json.Unmarshal(msg.Bytes(), &labels); err != nil {
		log.Println("failed to unmarshal labels", string(msg.Bytes()), err)
		return
	}
	log.Println("new connection from", conn.RemoteAddr(), labels)
	status := int32(200)
	dockerName, ok := labels["docker.name"]
	if !ok {
		status = 400
	}
	if err := sendResponse(conn, status, timeout); err != nil {
		log.Println("failed to write response", err)
		return
	}
	if status != 200 {
		return
	}

	f, err := os.OpenFile(path.Join(logDir, dockerName + ".log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	for {
		if err := readMsg(conn, msg, 0); err != nil {
			log.Println("failed to read msg from", conn.RemoteAddr(), err)
			return
		}
		log.Printf("got %d bytes from %s", msg.Len(), conn.RemoteAddr()) //todo
		if _, err := f.Write(msg.Bytes()); err != nil {
			sendResponse(conn, 500, timeout)
			return
		}
		if err := sendResponse(conn, 200, timeout); err != nil {
			log.Println("failed to write response", err)
			return
		}
	}
}

func main() {
	var logPath, listen string
	flag.StringVar(&logPath, "log-path", "", "absolute logs path")
	flag.StringVar(&listen, "listen", "", "listen address ip:port or :port")
	flag.Parse()

	if logPath == "" {
		log.Fatalln("-log-path argument isn't set")
	}
	if listen == "" {
		log.Fatalln("-listen argument isn't set")
	}
	log.Println("log path is", logPath)
	log.Println("listening on", listen)
	log.Panic(listenAndServe(listen, logPath))
}