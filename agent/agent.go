package agent

import (
	"time"
	"sync"
	"path/filepath"
	"log"
	"path"
	"github.com/prometheus/client_golang/prometheus"
	"fmt"
)

const (
	globRefreshInterval = 5 * time.Second
	bufferSize = 100000
	bufferTimeout = 10 * time.Second
	timeout = 10 * time.Second
)


var (
	logsCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:    "oklogging_agent_logs_count",
		Help:    "Tailing logs count",
	})
	bytesRead = prometheus.NewCounter(prometheus.CounterOpts{
		Name:    "oklogging_agent_bytes_read",
		Help:    "Bytes read from logs",
	})
	bytesWritten = prometheus.NewCounter(prometheus.CounterOpts{
		Name:    "oklogging_agent_bytes_written",
		Help:    "Bytes written to server",
	})
	writeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name:    "oklogging_agent_write_errors",
		Help:    "Write errors count",
	})
	writeOperations = prometheus.NewCounter(prometheus.CounterOpts{
		Name:    "oklogging_agent_write_ops",
		Help:    "Write ops count",
	})

	linesRead = prometheus.NewCounter(prometheus.CounterOpts{
		Name:    "oklogging_agent_lines_read",
		Help:    "Lines read from logs",
	})
	offsetsCommits = prometheus.NewCounter(prometheus.CounterOpts{
		Name:    "oklogging_agent_offsets_committed",
		Help:    "Offsets commits occurred",
	})
	jsonHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "oklogging_agent_json_transformer_histogram",
		Help:    "Json transformer histogram",
	})
	writeHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "oklogging_agent_write_histogram",
		Help:    "Write buffer to server histogram",
	})
)

func init(){
	prometheus.MustRegister(logsCount)
	prometheus.MustRegister(bytesRead)
	prometheus.MustRegister(bytesWritten)
	prometheus.MustRegister(writeErrors)
	prometheus.MustRegister(writeOperations)

	prometheus.MustRegister(linesRead)
	prometheus.MustRegister(offsetsCommits)
	prometheus.MustRegister(jsonHistogram)
	prometheus.MustRegister(writeHistogram)
}

type LogAgent struct {
	globPattern string
	lock sync.Mutex
	copiers map[string]*Copier
	offsetStorage *OffsetStorage
	server string
}

func NewLogAgent(dockerContainersDir string, offsetStoreDir string, server string) (*LogAgent, error) {
	if server == "" {
		return nil, fmt.Errorf("empty server")
	}
	logAgent :=  &LogAgent{
		globPattern: path.Join(dockerContainersDir, "*/*-json.log"),
		copiers: map[string]*Copier{},
		server: server,
	}
	var err error
	logAgent.offsetStorage, err = NewOffsetStorage(offsetStoreDir)
	if err != nil {
		return nil, err
	}
	err = logAgent.refreshGlob()
	if err != nil {
		return nil, err
	}
	return logAgent, nil
}

func (agent *LogAgent) Run() {
	ticker := time.NewTicker(globRefreshInterval).C
	for range ticker {
		err := agent.refreshGlob()
		if err != nil {
			log.Println("failed to refresh glob", agent.globPattern, err)
			continue
		}
	}
}

func (agent *LogAgent) Close() {
	agent.lock.Lock()
	defer agent.lock.Unlock()
	for file, copier := range agent.copiers {
		copier.Close()
		delete(agent.copiers, file)
	}
}

func (agent *LogAgent) refreshGlob() error {
	agent.lock.Lock()
	defer agent.lock.Unlock()

	files, err := filepath.Glob(agent.globPattern)
	if err != nil {
		return err
	}
	freshFiles := map[string]struct{}{}
	for _, f := range files {
		freshFiles[f] = struct{}{}
		if _, ok := agent.copiers[f]; ok {
			continue
		}
		labels, err := GetLabelsByLog(f)
		if err != nil {
			if err != ErrSkip {
				log.Println("failed to get labels for log", f, err)
			}
			continue
		}
		log.Println("got labels for log", f, labels)
		in, err := NewFileInput(f, agent.offsetStorage)
		if err != nil {
			log.Println("failed to init input", err)
			continue
		}
		out := NewTcpOutput(agent.server, labels, timeout)
		copier := NewCopier(in, out, &DockerJsonTransformer{}, bufferSize, bufferTimeout)
		go copier.Run()
		agent.copiers[f] = copier
	}
	for file, copier := range agent.copiers {
		if _, ok := freshFiles[file]; !ok {
			copier.Close()
			delete(agent.copiers, file)
		}
	}
	logsCount.Set(float64(len(agent.copiers)))
	agent.offsetStorage.GC(files)
	log.Println("files list refreshed")
	return nil
}