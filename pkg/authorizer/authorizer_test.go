// Copyright Jetstack Ltd. See LICENSE for details.

package authorizer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/authzcache"
	v1 "k8s.io/api/authorization/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type userInfoT struct {
	name   string
	groups []string
	extra  map[string][]string
}

func (u userInfoT) GetName() string               { return u.name }
func (u userInfoT) GetGroups() []string           { return u.groups }
func (u userInfoT) GetExtra() map[string][]string { return u.extra }
func (u userInfoT) GetUID() string                { return "xuy" }

type attrsType struct {
	namespace, verb, apiGroup, apiVersion, resource, subresource, name, path string
	user                                                                     userInfoT
	resourceRq                                                               bool
	readOnly                                                                 bool
}

func (a attrsType) GetNamespace() string    { return a.namespace }
func (a attrsType) GetVerb() string         { return a.verb }
func (a attrsType) GetAPIGroup() string     { return a.apiGroup }
func (a attrsType) GetAPIVersion() string   { return a.apiVersion }
func (a attrsType) GetResource() string     { return a.resource }
func (a attrsType) GetSubresource() string  { return a.subresource }
func (a attrsType) GetName() string         { return a.name }
func (a attrsType) GetPath() string         { return a.path }
func (a attrsType) GetUser() user.Info      { return a.user }
func (a attrsType) IsReadOnly() bool        { return a.readOnly }
func (a attrsType) IsResourceRequest() bool { return a.resourceRq }

var testAccess = attrsType{
	namespace:  "default",
	resource:   "pods",
	apiVersion: "v1",
	apiGroup:   "",
	user: userInfoT{
		name:   "testme",
		groups: []string{"developers", "readers"},
		extra: map[string][]string{
			"business":    {"sdvor"},
			"environment": {"stage", "production"},
		},
	},
}

func alwaysDeny(sar *v1.SubjectAccessReview, cache *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
	sar.Status.Denied = true
	sar.Status.Allowed = true
	sar.Status.Reason = "Go away"
	return sar, nil
}

func allowAccess(sar *v1.SubjectAccessReview, cache *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
	sar.Status.Denied = false
	sar.Status.Allowed = true
	sar.Status.Reason = ""
	return sar, nil
}

func notAllowed(sar *v1.SubjectAccessReview, cache *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
	sar.Status.Denied = false
	sar.Status.Allowed = false
	sar.Status.Reason = ""
	return sar, nil
}

func authzError(sar *v1.SubjectAccessReview, cache *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
	return sar, fmt.Errorf("oops, something goes wrong")
}

func TestDecision(t *testing.T) {
	authzer := NewOPAAuthorizer(nil, &options.AuthorizerOptions{AuthorizerUri: "localhost:8080"})
	decision, reason, err := authzer.authorize(context.Background(), testAccess, alwaysDeny)
	if err != nil {
		t.Error(err.Error())
	}
	if decision != authorizer.DecisionDeny && len(reason) == 0 {
		t.Error("Must be denied but not")
	}
	decision, reason, err = authzer.authorize(context.Background(), testAccess, allowAccess)
	if err != nil {
		t.Error(err.Error())
	}
	if decision != authorizer.DecisionAllow && len(reason) != 0 {
		t.Error("Must be allowed but not")
	}
	decision, _, err = authzer.authorize(context.Background(), testAccess, notAllowed)
	if err != nil {
		t.Error(err.Error())
	}
	if decision != authorizer.DecisionNoOpinion {
		t.Error("Must be no opinion but not")
	}
	_, _, err = authzer.authorize(context.Background(), testAccess, authzError)
	if err == nil {
		t.Error("must be error but no")
	}
}

func TestHttpAuthorizer(t *testing.T) {
	testURI := "http://127.0.0.1:32700"
	testUrlParsed, _ := url.Parse(testURI)
	testContext, cancelFn := context.WithCancel(context.Background())
	cache := authzcache.NewOPACache()
	go func(ctx context.Context) {
		mux := http.NewServeMux()
		srv := &http.Server{
			Addr:    testUrlParsed.Host,
			Handler: mux,
		}
		mux.Handle("/allow/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusOK)
			sar, _ := allowAccess(NewSubjectAccessReviewFromAttributes(testAccess), cache)
			response, _ := json.Marshal(opaResponse{Result: *sar})
			rw.Write(response)
		}))
		mux.Handle("/deny/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusOK)
			sar, _ := alwaysDeny(NewSubjectAccessReviewFromAttributes(testAccess), cache)
			response, _ := json.Marshal(opaResponse{Result: *sar})
			rw.Write(response)
		}))
		mux.Handle("/noopinion/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusOK)
			sar, _ := notAllowed(NewSubjectAccessReviewFromAttributes(testAccess), cache)
			response, _ := json.Marshal(opaResponse{Result: *sar})
			rw.Write(response)
		}))
		stopCh := ctx.Done()
		go srv.ListenAndServe()
		<-stopCh
	}(testContext)
	// wait server up and running
	for {
		_, err := net.DialTimeout("tcp", testUrlParsed.Host, time.Millisecond*500)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
	a := NewOPAAuthorizer(nil, &options.AuthorizerOptions{AuthorizerUri: testURI})
	a.cacher = authzcache.NewOPACache()
	a.opaURI = strings.Join([]string{testURI, "/404/"}, "")
	decision, _, err := a.Authorize(testContext, testAccess)
	if err == nil {
		t.Error("must be 404")
	}
	fmt.Println(err.Error())
	a.opaURI = strings.Join([]string{testURI, "/deny/"}, "")
	decision, _, err = a.Authorize(testContext, testAccess)
	if err != nil {
		t.Error(err.Error())
	}
	if decision != authorizer.DecisionDeny {
		t.Error("must be denied but no")
	}
	a.opaURI = strings.Join([]string{testURI, "/allow/"}, "")
	decision, _, err = a.Authorize(testContext, testAccess)
	if err != nil {
		t.Error(err.Error())
	}
	if decision != authorizer.DecisionAllow {
		t.Error("must be allowed but no")
	}

	cancelFn()
}
