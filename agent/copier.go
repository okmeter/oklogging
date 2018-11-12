package agent

import (
	"context"
	"bytes"
	"time"
	"log"
)

type Copier struct {
	input Input
	output Output
	transformer Transformer
	ctx context.Context
	cancelFn context.CancelFunc
	closed bool
	bufferSize int
	bufferTimeout time.Duration

}

func NewCopier(in Input, out Output, tr Transformer, bufferSize int, bufferTimeout time.Duration) *Copier {
	ctx, cancelFn := context.WithCancel(context.Background())
	return &Copier{
		input: in,
		output: out,
		transformer: tr,
		ctx: ctx,
		cancelFn: cancelFn,
		closed: false,
		bufferSize: bufferSize,
		bufferTimeout: bufferTimeout,
	}
}

func (c *Copier) Closed() bool {
	return c.closed
}

func (c *Copier) Close() {
	if c.closed {
		return
	}
	c.input.Close()
	c.output.Close()
	c.closed = true
	return
}

func (c *Copier) Run() {
	defer c.Close()
	buf := &bytes.Buffer{}
	flushTimer := time.NewTimer(c.bufferTimeout)

	flushBuffer := func() {
		if buf.Len() < 1 {
			return
		}
		flushTimer.Stop()
		flushTimer = time.NewTimer(c.bufferTimeout)
		writeOperations.Inc()
		err := c.output.Write(buf.Bytes())
		if err != nil {
			log.Println("failed to write to output", c.output, err)
			writeErrors.Inc()
			time.Sleep(time.Second) //todo
			return
		}
		if err = c.input.SaveOffset(); err != nil {
			log.Println("failed to save input offset", err)
		}
		buf.Reset()
	}

	for {
		select {
		case <- c.ctx.Done():
			return
		case <- flushTimer.C:
			flushBuffer()
		default:
			if buf.Len() >= c.bufferSize {
				flushBuffer()
				continue
			}
			line, err := c.input.ReadLine()
			if err != nil {
				//todo: logging
				return
			}
			line, err = c.transformer.Do(line)
			if err != nil {
				//todo: logging
				continue
			}
			buf.WriteString(line)
		}
	}
}

