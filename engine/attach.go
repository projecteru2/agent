package engine

import (
	"bufio"
	"context"
	"io"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/docker/docker/pkg/stdcopy"

	log "github.com/Sirupsen/logrus"
	dockertypes "github.com/docker/docker/api/types"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/logs"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/watcher"
)

func (e *Engine) attach(container *types.Container, stop chan int) {
	transfer := e.forwards.Get(container.ID, 0)
	writer, err := logs.NewWriter(transfer, e.config.Log.Stdout)
	if err != nil {
		log.Errorf("Create log forward failed %s", err)
		return
	}

	outr, outw := io.Pipe()
	errr, errw := io.Pipe()
	go func() {
		ctx := context.Background()
		options := dockertypes.ContainerAttachOptions{
			Stream: true,
			Stdin:  false,
			Stdout: true,
			Stderr: true,
		}
		resp, err := e.docker.ContainerAttach(ctx, container.ID, options)
		if err != nil && err != httputil.ErrPersistEOF {
			log.Errorf("attach %s container %s failed %s", container.Name, container.ID[:7], err)
			return
		}
		defer resp.Close()
		defer outw.Close()
		defer errw.Close()
		_, err = stdcopy.StdCopy(outw, errw, resp.Reader)
		log.Infof("attach %s container %s finished", container.Name, container.ID[:7])
		stop <- 1
		if err != nil {
			log.Errorf("attach get stream failed %s", err)
		}
	}()
	log.Infof("attach %s container %s success", container.Name, container.ID[:7])
	pump := func(typ string, source io.Reader) {
		buf := bufio.NewReader(source)
		for {
			data, err := buf.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Errorf("attach pump %s %s %s %s", container.Name, container.ID[:7], typ, err)
				}
				return
			}
			data = strings.TrimSuffix(data, "\n")
			data = strings.TrimSuffix(data, "\r")
			l := &types.Log{
				ID:         container.ID,
				Name:       container.Name,
				Type:       typ,
				EntryPoint: container.EntryPoint,
				Ident:      container.Ident,
				Data:       data,
				Datetime:   time.Now().Format(common.DATETIME_FORMAT),
				Zone:       e.config.Zone,
			}
			watcher.LogMonitor.LogC <- l
			if err := writer.Write(l); err != nil {
				log.Errorf("%s container %s write failed %s", container.Name, container.ID[:7], err)
				log.Error(data)
			}
		}
	}
	go pump("stdout", outr)
	go pump("stderr", errr)
}
