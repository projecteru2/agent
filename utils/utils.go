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

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/version"
	coreutils "github.com/projecteru2/core/utils"
	yavirtclient "github.com/projecteru2/libyavirt/client"

	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/log"
)

var dockerized bool
var once sync.Once

// MakeDockerClient make a docker client
func MakeDockerClient(config *types.Config) (*engineapi.Client, error) {
	defaultHeaders := map[string]string{"User-Agent": fmt.Sprintf("eru-agent-%s", version.VERSION)}
	return engineapi.NewClient(config.Docker.Endpoint, common.DockerCliVersion, nil, defaultHeaders)
}

// MakeYavirtClient make a yavirt client
func MakeYavirtClient(config *types.Config) (yavirtclient.Client, error) {
	return yavirtclient.New(config.Yavirt.Endpoint)
}

// WritePid write pid
func WritePid(path string) {
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		log.Fatalf(nil, err, "Save pid file failed %s", err) //nolint
	}
}

// GetAppInfo return app info
func GetAppInfo(containerName string) (name, entrypoint, ident string, err error) {
	return coreutils.ParseWorkloadName(containerName)
}

// UseLabelAsFilter return if use label as filter
func UseLabelAsFilter() bool {
	return os.Getenv("ERU_AGENT_EXPERIMENTAL_FILTER") == "label"
}

// GetMaxAttemptsByTTL .
func GetMaxAttemptsByTTL(ttl int64) int {
	// if selfmon is enabled, retry 5 times
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

	// deal with "legal" error rune in utf8
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

// IsDockerized returns if the agent is running in docker
func IsDockerized() bool {
	once.Do(func() {
		dockerized = os.Getenv(common.DOCKERIZED) != ""
	})
	return dockerized
}

// WithTimeout runs a function with given timeout
func WithTimeout(ctx context.Context, timeout time.Duration, f func(ctx2 context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f(ctx)
}

// GetIP Get hostIP
func GetIP(daemonHost string) string {
	u, err := url.Parse(daemonHost)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
