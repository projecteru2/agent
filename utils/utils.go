package utils

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	coreutils "github.com/projecteru2/core/utils"
	yavirtclient "github.com/projecteru2/libyavirt/client"
	yavirttypes "github.com/projecteru2/libyavirt/types"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/version"

	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/log"
)

var (
	dockerized bool
	once       sync.Once
)

func MakeDockerClient(config *types.Config) (*engineapi.Client, error) {
	return engineapi.NewClientWithOpts(
		engineapi.WithHost(config.Docker.Endpoint),
		engineapi.WithVersion(common.DockerCliVersion),
		engineapi.WithHTTPHeaders(map[string]string{"User-Agent": "eru-agent-" + version.VERSION}),
	)
}

func MakeYavirtClient(config *types.Config) (yavirtclient.Client, error) {
	return yavirtclient.New(&yavirttypes.Config{URI: config.Yavirt.Endpoint})
}

func WritePid(path string) {
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		log.Fatalf(nil, err, "Save pid file failed %s", err) //nolint
	}
}

func GetAppInfo(containerName string) (name, entrypoint, ident string, err error) {
	return coreutils.ParseWorkloadName(containerName)
}

func UseLabelAsFilter() bool {
	return os.Getenv("ERU_AGENT_EXPERIMENTAL_FILTER") == "label"
}

func GetMaxAttemptsByTTL(ttl int64) int {
	// a zero ttl means core owns expiry, so use a fixed attempt count
	if ttl < 1 {
		return 5
	}
	return int(math.Floor(math.Log2(float64(ttl)+1))) + 1
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

func WithTimeout(ctx context.Context, timeout time.Duration, f func(ctx2 context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f(ctx)
}

func GetIP(daemonHost string) string {
	u, err := url.Parse(daemonHost)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
