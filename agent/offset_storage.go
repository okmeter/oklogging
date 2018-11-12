package agent

import (
	"os"
	"fmt"
	"log"
	"io/ioutil"
	"path"
	"strings"
	"strconv"
)

type OffsetStorage struct {
	basePath string
}

func NewOffsetStorage(basePath string) (*OffsetStorage, error) {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		if err := os.Mkdir(basePath, os.FileMode(0775)); err != nil {
			return nil, fmt.Errorf(`failed to create offset storage dir: %s`, err)
		} else {
			log.Println("creating offset storage dir", basePath)
		}
	}
	return &OffsetStorage{basePath: basePath}, nil
}

func (storage *OffsetStorage) GC(freshFiles []string) {
	freshKeys := make(map[string]struct{}, len(freshFiles))
	for _, f := range freshFiles {
		freshKeys[storage.key(f)] = struct{}{}
	}
	actualFiles, err := ioutil.ReadDir(storage.basePath)
	if err != nil {
		log.Println(err)
		return
	}
	for _, fi := range actualFiles {
		if _, ok := freshKeys[fi.Name()]; !ok {
			log.Println("removing offset file", fi.Name())
			os.Remove(path.Join(storage.basePath, fi.Name()))
		}
	}
}

func (storage *OffsetStorage) key(f string) string {
	return strings.Replace(f, "/", "_", -1)
}

func (storage *OffsetStorage) offsetPath(f string) string {
	return path.Join(storage.basePath, storage.key(f))
}

func (storage *OffsetStorage) Get(f string) (int64, error) {
	offsetFilePath := storage.offsetPath(f)
	data, err := ioutil.ReadFile(offsetFilePath)
	if err != nil {
		return -1, err
	}
	offset, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		log.Println("got invalid offset from", offsetFilePath, err)
		os.Remove(offsetFilePath)
		return -1, err
	}
	return offset, nil
}

func (storage *OffsetStorage) Save(f string, offset int64) error {
	offsetFilePath := storage.offsetPath(f)
	return ioutil.WriteFile(offsetFilePath, []byte(fmt.Sprintf("%d", offset)), 0644)
}