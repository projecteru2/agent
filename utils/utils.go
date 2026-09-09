package utils

import (
	"context"
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

	"github.com/projecteru2/agent/common"
)

const (
	// CgroupRoot is where the unified cgroup v2 hierarchy is mounted.
	CgroupRoot = "/sys/fs/cgroup"

	hexDigits = "0123456789abcdef"
)

var (
	isDockerized     = sync.OnceValue(func() bool { return os.Getenv(common.DOCKERIZED) != "" })
	useLabelAsFilter = sync.OnceValue(labelFilterEnabled)
)

func WritePid(ctx context.Context, path string) {
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		log.Fatalf(ctx, err, "save pid file %s", path)
	}
}

func UseLabelAsFilter() bool {
	return useLabelAsFilter()
}

// ReplaceNonUtf8 replaces non-utf8 characters in \x format.
func ReplaceNonUtf8(str string) string {
	if str == "" {
		return str
	}
	replaceable := strings.Contains(str, string(utf8.RuneError))
	if utf8.ValidString(str) && !replaceable {
		return str
	}

	// U+FFFD may be a legitimate rune, escape it before validating
	if replaceable {
		str = strings.ReplaceAll(str, string(utf8.RuneError), "\\xef\\xbf\\xbd")
	}

	if utf8.ValidString(str) {
		return str
	}

	var v strings.Builder
	v.Grow(len(str))
	for i, r := range str {
		switch {
		case r == utf8.RuneError:
			writeEscaped(&v, rune(str[i]))
		case unicode.IsControl(r) && r != '\r' && r != '\n':
			writeEscaped(&v, r)
		default:
			v.WriteRune(r)
		}
	}
	return v.String()
}

// ProcRoot returns where this agent reads the host's procfs.
func ProcRoot() string {
	if isDockerized() {
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

func labelFilterEnabled() bool {
	return os.Getenv("ERU_AGENT_EXPERIMENTAL_FILTER") == "label"
}

func writeEscaped(v *strings.Builder, c rune) {
	v.WriteString("\\x")
	v.WriteByte(hexDigits[c>>4])
	v.WriteByte(hexDigits[c&0xf])
}
