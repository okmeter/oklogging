package main

import (
	".."
	"log"
	"flag"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func main() {
	var containersDir, offsetsDir, server, metricsListen string
	flag.StringVar(&containersDir, "containers-dir", "/var/lib/docker/containers", "containers path")
	flag.StringVar(&offsetsDir, "offsets-dir", "", "offsets save dir")
	flag.StringVar(&metricsListen, "metricsListen", "", "ip:port of :port for /metrics")
	flag.StringVar(&server, "server", "", "server ip:port")
	flag.Parse()
	if server == "" {
		log.Fatalln("-server argument isn't set")
	}
	if offsetsDir == "" {
		log.Fatalln("-offsets-dir argument isn't set")
	}
	loggingAgent, err := agent.NewLogAgent(containersDir, offsetsDir, server)
	if err != nil {
		log.Fatalln("failed to init agent:", err)
	}

	http.Handle("/metrics", promhttp.Handler())
	if metricsListen != "" {
		go func() {
			log.Println("listening for /metrics on", metricsListen)
			log.Fatal(http.ListenAndServe(metricsListen, nil))
		}()
	}
	loggingAgent.Run()
}