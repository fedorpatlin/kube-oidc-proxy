// Copyright Jetstack Ltd. See LICENSE for details.

package authorizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/authzcache"
	"github.com/taskcluster/httpbackoff"
	v1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sJson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog"
)

// Open Policy Agent authorizer
type OPAAuthorizer struct {
	opaURI string
	cacher *authzcache.OPACache
}
type opaResponse struct {
	Result v1.SubjectAccessReview
}

func NewOPAAuthorizer(opts *options.AuthorizerOptions) *OPAAuthorizer {
	return &OPAAuthorizer{opaURI: opts.AuthorizerUri}
}

func convertToV1Authz(userExtras map[string][]string) map[string]v1.ExtraValue {
	extraValues := map[string]v1.ExtraValue{}
	for k, v := range userExtras {
		extraValues[k] = v
	}
	return extraValues
}

func NewSubjectAccessReviewFromAttributes(attrs authorizer.Attributes) *v1.SubjectAccessReview {
	userExtraValues := convertToV1Authz(attrs.GetUser().GetExtra())
	sar := &v1.SubjectAccessReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SubjectAccessReview",
			APIVersion: "authorization.k8s.io/v1",
		},
		Spec: v1.SubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace:   attrs.GetNamespace(),
				Verb:        attrs.GetVerb(),
				Group:       attrs.GetAPIGroup(),
				Version:     attrs.GetAPIVersion(),
				Resource:    attrs.GetResource(),
				Subresource: attrs.GetSubresource(),
				Name:        attrs.GetName(),
			},
			NonResourceAttributes: &v1.NonResourceAttributes{
				Path: attrs.GetPath(),
				Verb: attrs.GetVerb(),
			},
			User:   attrs.GetUser().GetName(),
			Groups: attrs.GetUser().GetGroups(),
			Extra:  userExtraValues,
		},
	}
	return sar
}

func (a *OPAAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return a.authorize(ctx, attrs, authzRequestFunc(a.opaURI))
}

func (a *OPAAuthorizer) authorize(ctx context.Context, attrs authorizer.Attributes, authzFn func(*v1.SubjectAccessReview, *authzcache.OPACache) (*v1.SubjectAccessReview, error)) (authorizer.Decision, string, error) {
	sar := NewSubjectAccessReviewFromAttributes(attrs)
	// request authorizer
	responseSAR, err := authzFn(sar, a.cacher)
	if err != nil {
		return authorizer.DecisionNoOpinion, "I have no idea about it", err
	}
	if responseSAR.Status.Denied {
		return authorizer.DecisionDeny, responseSAR.Status.Reason, nil
	}

	if !responseSAR.Status.Allowed {
		return authorizer.DecisionNoOpinion, responseSAR.Status.Reason, nil
	}
	if a.cacher != nil {
		cachePositive, err := json.Marshal(responseSAR)
		if err == nil {
			a.cacher.Put(string(createOpaRequestPayload(sar)), &cachePositive)
		} else {
			klog.Errorf("error marshaling SAR: %s", err.Error())
		}
	}
	return authorizer.DecisionAllow, "", nil
}

func authzRequestFunc(uri string) func(*v1.SubjectAccessReview, *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
	return func(sar *v1.SubjectAccessReview, cache *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
		var resp opaResponse
		jsonPayload := createOpaRequestPayload(sar)
		if cache != nil {
			bytes, ok := cache.Get(string(jsonPayload))
			if ok {
				cachedResponse := &v1.SubjectAccessReview{}
				err := json.Unmarshal(*bytes, cachedResponse)
				if err == nil {
					return cachedResponse, nil
				}
			}
		}
		backoffClient := httpbackoff.Client{BackOffSettings: backoff.NewExponentialBackOff()}
		backoffClient.BackOffSettings.MaxElapsedTime = time.Second * 5
		authzResponse, _, err := backoffClient.Post(uri, "application/json", jsonPayload)
		if err != nil {
			klog.Errorf("Authorization server is not responding: %s", err.Error())
			return nil, err
		}
		defer authzResponse.Body.Close()
		// if authzResponse.StatusCode != 200 {
		// 	return nil, fmt.Errorf("response code not HTTP.OK: %d", authzResponse.StatusCode)
		// }
		bodyBytes := make([]byte, 2048)
		bytesRead, err := authzResponse.Body.Read(bodyBytes)
		if err != nil && err != io.EOF {
			klog.Errorf("Error reading Authz response body: %s", err.Error())
			return nil, err
		}
		bodyBytes = bodyBytes[:bytesRead]
		err = json.Unmarshal(bodyBytes, &resp)
		return &resp.Result, err
	}
}

func createOpaRequestPayload(sar *v1.SubjectAccessReview) []byte {
	sarSerializer := k8sJson.NewSerializerWithOptions(k8sJson.DefaultMetaFactory, nil, nil, k8sJson.SerializerOptions{
		Yaml:   false,
		Pretty: false,
		Strict: true,
	})
	var buf []byte
	serialisedSAR := bytes.NewBuffer(buf)
	sarSerializer.Encode(sar, serialisedSAR)
	jsonPayload := fmt.Sprintf("{\"input\": %s}", serialisedSAR.Bytes())
	return []byte(jsonPayload)
}
