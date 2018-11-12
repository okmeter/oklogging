package agent

import (
	"gopkg.in/okmeter/gotail.v1"
	"context"
	"log"
	"os"
)

type Input interface {
	Close()
	ReadLine() (string, error)
	SaveOffset() error
}

type FileInput struct {
	filePath string
	tail *tail.Tail
	ctx context.Context
	cancelFn context.CancelFunc
	offsetStorage *OffsetStorage
}

func NewFileInput(filePath string, offsetStorage *OffsetStorage) (*FileInput, error) {
	fi := &FileInput{
		offsetStorage: offsetStorage,
		filePath: filePath,
	}

	offset, err := offsetStorage.Get(filePath)
	if err != nil {
		log.Println("can't get offset for file", filePath, err)
		offset = 0
	} else {
		fi, err := os.Stat(filePath)
		if err != nil {
			return nil, err
		}
		if fi.Size() < offset {
			offset = 0
		}
	}
	log.Println("tailing file", filePath, "from offset", offset)
	t, err := tail.NewTail(filePath, offset, tail.NewConfig())
	if err != nil {
		return nil, err
	}
	fi.tail = &t
	fi.ctx, fi.cancelFn = context.WithCancel(context.Background())
	return fi, nil
}

func (fi *FileInput) Close() {
	log.Println("closing fileinput for", fi.filePath)
	fi.tail.Close()
	fi.cancelFn()
}

func (fi *FileInput) ReadLine() (string, error) {
	select {
	case <- fi.ctx.Done():
		return "", fi.ctx.Err()
	default:
		line, err := fi.tail.ReadLine()
		if err != nil {
			log.Println(err)
			return "", err
		}
		linesRead.Inc()
		bytesRead.Add(float64(len(line)+1))
		return line, nil
	}
}

func (fi *FileInput) SaveOffset() error {
	offset, err := fi.tail.Offset()
	if err != nil {
		return err
	}
	offsetsCommits.Inc()
	return fi.offsetStorage.Save(fi.filePath, offset)
}