package pollers

import (
	"github.com/freeformz/shh/config"
	"github.com/freeformz/shh/mm"
	"github.com/freeformz/shh/utils"
	"strings"
	"syscall"
	"time"
)

type Df struct {
	measurements chan<- *mm.Measurement
}

func NewDfPoller(measurements chan<- *mm.Measurement) Df {
	return Df{measurements: measurements}
}

func (poller Df) Poll(tick time.Time) {
	ctx := utils.Slog{"poller": poller.Name(), "fn": "Poll", "tick": tick}

	buf := new(syscall.Statfs_t)

	for mp := range mountpointChannel() {
		err := syscall.Statfs(mp, buf)
		if err != nil {
			ctx["mountpoint"] = mp
			ctx.FatalError(err, "calling Statfs")
		}
		mmp := massageMountPoint(mp)
		total_bytes := float64(uint64(buf.Bsize) * buf.Blocks)
		poller.measurements <- &mm.Measurement{tick, poller.Name(), []string{mmp, "total_bytes"}, total_bytes}
		user_free_bytes := float64(uint64(buf.Bsize) * buf.Bavail)
		root_free_bytes := float64(uint64(buf.Bsize)*buf.Bfree) - user_free_bytes
		poller.measurements <- &mm.Measurement{tick, poller.Name(), []string{mmp, "root", "free", "bytes"}, root_free_bytes}
		poller.measurements <- &mm.Measurement{tick, poller.Name(), []string{mmp, "user", "free", "bytes"}, user_free_bytes}
		poller.measurements <- &mm.Measurement{tick, poller.Name(), []string{mmp, "used", "bytes"}, total_bytes - root_free_bytes - user_free_bytes}
		poller.measurements <- &mm.Measurement{tick, poller.Name(), []string{mmp, "total", "inodes"}, float64(buf.Files)}
		poller.measurements <- &mm.Measurement{tick, poller.Name(), []string{mmp, "free", "inodes"}, float64(buf.Ffree)}
	}
}

func (poller Df) Name() string {
	return "df"
}
func (poller Df) Exit() {}

// Utility functions
// Massages the mount point so that "/" == "root" and
// other substitutions
func massageMountPoint(path string) string {
	switch path {
	case "/":
		return "root"
	}
	if strings.HasPrefix(path, "/") {
		path = strings.TrimLeft(path, "/")
	}
	path = strings.Replace(path, "/", "_", -1)
	return path
}

// Returns a channel on which you can receive the mountspoints we care about
func mountpointChannel() <-chan string {
	c := make(chan string)

	go func(mountpoints chan<- string) {
		defer close(mountpoints)

		for line := range utils.FileLineChannel("/proc/mounts") {

			fields := strings.Fields(line)
			fsType := fields[2]

			if utils.SliceContainsString(config.DfTypes, fsType) {
				mountpoints <- fields[1]
			}
		}
	}(c)

	return c
}
