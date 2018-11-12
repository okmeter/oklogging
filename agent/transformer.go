package agent

import (
	"encoding/json"
	"time"
)

type Transformer interface {
	Do(string) (string, error)
}

type DockerJsonTransformer struct {}

type DockerLogJson struct {
	Log string
}

func (j *DockerJsonTransformer) Do(line string) (string, error) {
	start := time.Now()
	obj := DockerLogJson{}
	err := json.Unmarshal([]byte(line), &obj)
	jsonHistogram.Observe(time.Since(start).Seconds())
	return obj.Log, err
}

type PassThroughTransformer struct {}

func (t *PassThroughTransformer) Do(line string) (string, error) {
	return line, nil
}

