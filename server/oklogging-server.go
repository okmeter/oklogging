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
	"io/ioutil"
	"regexp"
)

const (
	timeout = 10 * time.Second
	maxLogSize = 1 * 1024 * 1024 * 1024
	backupLogDateFormat = "2006-01-02T15-04-05.000"
)

var (
	backupFilePattern = regexp.MustCompile(`.+-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3}\.log$`)
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

	logPath := path.Join(logDir, dockerName + ".log")
	currentSize := int64(0)

	if fi, err := os.Stat(logPath); err == nil {
		currentSize = fi.Size()
		if currentSize >= maxLogSize {
			err := os.Rename(logPath,
				path.Join(logDir, dockerName + "-" + time.Now().Format(backupLogDateFormat)+ ".log"))
			if err != nil {
				log.Println("failed to move log", err)
			}
			return
		}
	}

	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
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
		currentSize += int64(msg.Len())
		if currentSize >= maxLogSize {
			log.Printf("closing connection with %s for log rotation", conn.RemoteAddr())
			return
		}
	}
}

func gc(logPath string, maxAge time.Duration) {
	log.Println("GC started")
	now := time.Now()
	files, err := ioutil.ReadDir(logPath)
	if err != nil {
		log.Println(err)
		return
	}
	for _, f := range files {
		if !backupFilePattern.MatchString(f.Name()) {
			continue
		}
		if f.ModTime().Before(now.Add(-maxAge)) {
			log.Println("removing log", f.Name(), f.ModTime())
			if err := os.Remove(path.Join(logPath, f.Name())); err != nil {
				log.Println(err)
				continue
			}
		}
	}
	log.Println("GC finished in", time.Since(now).Seconds(), "seconds")
}


func main() {
	var logPath, listen string
	var maxAge time.Duration
	flag.StringVar(&logPath, "log-path", "", "absolute logs path")
	flag.StringVar(&listen, "listen", "", "listen address ip:port or :port")
	flag.DurationVar(&maxAge, "max-age", 3 * 24 * time.Hour, "time to retain old logs based on last file modification time")
	flag.Parse()

	if logPath == "" {
		log.Fatalln("-log-path argument isn't set")
	}
	if listen == "" {
		log.Fatalln("-listen argument isn't set")
	}
	log.Println("log path is", logPath)
	log.Println("listening on", listen)

	go func(){
		gc(logPath, maxAge)
		ticker := time.NewTicker(time.Minute * 10).C
		for range ticker {
			gc(logPath, maxAge)
		}
	}()
	log.Panic(listenAndServe(listen, logPath))
}