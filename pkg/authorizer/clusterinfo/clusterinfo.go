// Copyright Jetstack Ltd. See LICENSE for details.

package clusterinfo

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const extraBytesBufferSize = 1024

const (
	extrasSourceFile  = "file"
	extrasSourceHttp  = "http"
	extrasSourceHttps = "https"
)

type ClusterInfo map[string][]string

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
