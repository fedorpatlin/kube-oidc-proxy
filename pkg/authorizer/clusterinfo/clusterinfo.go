// Copyright Jetstack Ltd. See LICENSE for details.

package clusterinfo

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

const extraBytesBufferSize = 1024

const (
	extrasSourceFile  = "file"
	extrasSourceHttp  = "http"
	extrasSourceHttps = "https"
)

type ClusterInfo map[string][]string

func (c ClusterInfo) GetInfo() map[string][]string {
	return c
}

func FromUrl(userExtrasUrl string, prefix string) (ue *ClusterInfo, err error) {
	prefix = sanitize(prefix)
	userExtrasUrl = sanitize(userExtrasUrl)
	parsedUrl, err := url.Parse(userExtrasUrl)
	if err != nil {
		return nil, err
	}
	if parsedUrl.Scheme == extrasSourceFile || parsedUrl.Scheme == "" {
		return ReadFromFile(parsedUrl.Path, prefix)
	}
	err = fmt.Errorf("not yet implemented: %s", parsedUrl.Scheme)
	return nil, err
}

func ReadFromFile(fname string, prefix string) (ue *ClusterInfo, err error) {
	ue = new(ClusterInfo)
	extrasFile, err := readFile(fname)
	if err != nil {
		return nil, err
	}
	defer func() {
		ferr := extrasFile.Close()
		err = ferr
	}()
	ue = parseFromStrings(extrasFile, prefix)
	return
}

func readFile(fname string) (io.ReadCloser, error) {
	fname = filepath.Clean(filepath.Join("/", fname))
	extrasFile, err := os.OpenFile(fname, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return extrasFile, nil
}

func parseFromStrings(r io.Reader, prefix string) (ue *ClusterInfo) {
	ue = &ClusterInfo{}
	bufferedFile := bufio.NewReaderSize(r, extraBytesBufferSize)
	for {
		// annotation: some.prefix/name=value
		extrasLine, err := bufferedFile.ReadString('\n')
		if err != nil {
			break
		}
		// klog.Error(extrasLine)
		extrasLine = strings.Trim(extrasLine, "\"'")
		// malformed annotation
		if !strings.Contains(extrasLine, "=") {
			continue
		}
		if strings.HasPrefix(extrasLine, prefix) {
			result := strings.Split(strings.TrimPrefix(extrasLine, prefix), "=")
			if len(result) == 0 {
				return nil
			}
			(*ue)[strings.Trim(strings.TrimSpace(result[0]), "\"")] = []string{strings.Trim(strings.TrimSpace(result[1]), "\"")}
		}
	}
	return
}

func sanitize(userInput string) string {
	return strings.Trim(userInput, "\"'")
}

func (c ClusterInfo) WithClusterInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		requestUser, ok := request.UserFrom(r.Context())
		if ok {
			oldExtra := requestUser.GetExtra()
			if len(oldExtra) == 0 {
				oldExtra = make(map[string][]string)
			}
			newUser := &customUserInfo{Info: requestUser, customExtra: oldExtra}
			newUser.addExtra(c.GetInfo())
			r = r.WithContext(request.WithUser(r.Context(), newUser))
		}
		next.ServeHTTP(rw, r)
	})
}

type customUserInfo struct {
	user.Info
	customExtra map[string][]string
}

func (u *customUserInfo) GetExtra() map[string][]string {
	return u.customExtra
}

func (u *customUserInfo) addExtra(clusterInfo map[string][]string) {
	for k, v := range clusterInfo {
		u.customExtra[k] = v
	}
}
