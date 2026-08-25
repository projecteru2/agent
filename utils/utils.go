package utils

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/projecteru2/core/log"
	coreutils "github.com/projecteru2/core/utils"

	"github.com/projecteru2/agent/common"
)

// CgroupRoot is where the unified cgroup v2 hierarchy is mounted.
const CgroupRoot = "/sys/fs/cgroup"

var (
	dockerized bool
	once       sync.Once
)

func WritePid(ctx context.Context, path string) {
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		log.Fatalf(ctx, err, "save pid file %s", path)
	}
}

func GetAppInfo(containerName string) (name, entrypoint, ident string, err error) {
	return coreutils.ParseWorkloadName(containerName)
}

func UseLabelAsFilter() bool {
	return os.Getenv("ERU_AGENT_EXPERIMENTAL_FILTER") == "label"
}

// GetMaxAttemptsByTTL is fixed: core owns status expiry, so every call site passes a zero ttl.
func GetMaxAttemptsByTTL(ttl int64) int {
	return 5
}

// ReplaceNonUtf8 replaces non-utf8 characters in \x format.
func ReplaceNonUtf8(str string) string {
	if str == "" {
		return str
	}

	// U+FFFD may be a legitimate rune, escape it before validating
	if strings.ContainsRune(str, utf8.RuneError) {
		str = strings.ReplaceAll(str, string(utf8.RuneError), "\\xff\\xfd")
	}

	if utf8.ValidString(str) {
		return str
	}

	v := make([]rune, 0, len(str))
	for i, r := range str {
		switch {
		case r == utf8.RuneError:
			_, size := utf8.DecodeRuneInString(str[i:])
			if size > 0 {
				v = append(v, []rune(fmt.Sprintf("\\x%02x", str[i:i+size]))...)
			}
		case unicode.IsControl(r) && r != '\r' && r != '\n':
			v = append(v, []rune(fmt.Sprintf("\\x%02x", r))...)
		default:
			v = append(v, r)
		}
	}
	return string(v)
}

func IsDockerized() bool {
	once.Do(func() {
		dockerized = os.Getenv(common.DOCKERIZED) != ""
	})
	return dockerized
}

// ProcRoot returns where this agent reads the host's procfs.
func ProcRoot() string {
	if IsDockerized() {
		return "/hostProc"
	}
	return "/proc"
}

// CgroupPath returns the absolute cgroup v2 directory of the process pid.
func CgroupPath(cgroupRoot, procRoot string, pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cgroup")) //nolint:gosec // the pid comes from the runtime, never from a request
	if err != nil {
		return "", err
	}
	// cgroup v2 shows one "0::<path>" line, v1 shows a numbered line per controller
	for line := range strings.Lines(string(data)) {
		if rel, ok := strings.CutPrefix(strings.TrimSpace(line), "0::"); ok {
			return filepath.Join(cgroupRoot, rel), nil
		}
	}
	return "", common.ErrNoCgroupV2
}

func WithTimeout(ctx context.Context, timeout time.Duration, f func(ctx2 context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f(ctx)
}

// GetIP returns the host of a node endpoint, which is the only address a node record carries.
func GetIP(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
