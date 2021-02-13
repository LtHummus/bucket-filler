package streamer

import (
	"bufio"
	"io"
	"os/exec"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type reader struct {
	r io.Reader
}

func (r *reader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func captureOutput(r io.Reader) {
	reader := bufio.NewReader(r)
	var line string
	var err error
	for {
		line, err = reader.ReadString('\n')
		if err != nil && err != io.EOF {
			log.WithError(err).Warn("unable to read ffmpeg output")
			break
		}
		line = strings.TrimSpace(line)
		if line != "" {
			log.Warn(line)
		}
		if err != nil {
			break
		}
	}
}

func StartFfmpegRemux(ffmpegPath string, videoInput string, destinationFile string) error {
	remuxLogger := log.WithFields(log.Fields{
		"videoInput":      videoInput,
		"destinationFile": destinationFile,
	})
	remuxLogger.Info("starting remux")

	var command = []string{
		"-loglevel",
		"warning",
		"-hide_banner",
		"-i",
		videoInput,
		"-c",
		"copy",
		"-f",
		"flv",
		destinationFile,
	}

	r := exec.Command(ffmpegPath, command...)

	stderr, err := r.StderrPipe()
	if err != nil {
		remuxLogger.WithError(err).Warn("could not open stderr")
		return err
	}

	if err = r.Start(); err != nil {
		remuxLogger.WithError(err).Fatal("error starting ffmpeg")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		captureOutput(stderr)
		wg.Done()
	}()

	wg.Wait()
	if err != nil {
		remuxLogger.WithError(err).Fatal("error on wait")
	}

	remuxLogger.Info("remux complete")
	return nil
}
