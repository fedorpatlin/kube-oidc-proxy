// Copyright Jetstack Ltd. See LICENSE for details.

package noimpersonatedrequest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
)

func Test_header_added(t *testing.T) {
	rq := &http.Request{}
	rq.Header = map[string][]string{}
	rq = context.WithNoImpersonation(rq)
	WithPodSA(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(rq.Header.Get("Authorization"), "Bearer") {
			t.Fail()
		}
	}), func() []byte { return []byte("xuy") }).ServeHTTP(nil, rq)
}
