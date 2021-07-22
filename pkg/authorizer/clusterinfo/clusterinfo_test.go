// Copyright Jetstack Ltd. See LICENSE for details.

package clusterinfo

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

var extrasFileContent = `cluster-info.itlabs.io/business="all"
cluster-info.itlabs.io/environment="development"
`

var testClusterInfo = &ClusterInfo{
	"business":    {"all"},
	"environment": {"development"},
}

func TestExtrasFromStrings(t *testing.T) {
	buf := bytes.NewBufferString(extrasFileContent)
	ue := parseFromStrings(buf, "cluster-info.itlabs.io/")
	if ue == nil {
		t.FailNow()
	}
	business, ok := (*ue)["business"]
	if !ok {
		t.Fatalf("%v", ue)
	}
	if len(business) == 0 {
		t.Fatalf("no extra for business")
	}
	if business[0] != "all" {
		t.Fatalf("business must be \"all\" but it is %s", business[0])
	}
	environment := (*ue)["environment"]
	if environment[0] != "development" {
		t.FailNow()
	}
}

func TestExtrasFromUrl(t *testing.T) {
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		t.FailNow()
	}
	defer func() {
		err := tempFile.Close()
		log.Print(fmt.Errorf("%s %s", err, os.Remove(tempFile.Name())).Error())
	}()
	buf := bytes.NewBufferString(extrasFileContent)
	_, err = io.Copy(tempFile, buf)
	if err != nil {
		t.FailNow()
	}
	ue, err := FromUrl(strings.Join([]string{"file://", tempFile.Name()}, ""), "cluster-info.itlabs.io/")
	if err != nil {
		t.Fatal(err.Error())
	}
	if ue == nil {
		t.FailNow()
	}
	business, ok := (*ue)["business"]
	if !ok {
		t.Fatalf("%v", ue)
	}
	if len(business) == 0 {
		t.Fatalf("no extra for business")
	}
	if business[0] != "all" {
		t.Fatalf("business must be \"all\" but it is %s", business[0])
	}
	environment := (*ue)["environment"]
	if environment[0] != "development" {
		t.FailNow()
	}

}
