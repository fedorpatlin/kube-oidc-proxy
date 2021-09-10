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
	"github.com/jetstack/kube-oidc-proxy/pkg/authorizer/authzcache"
	"github.com/jetstack/kube-oidc-proxy/pkg/authorizer/clusterinfo"
	"github.com/taskcluster/httpbackoff"
	v1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sJson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

// Open Policy Agent authorizer
type OPAAuthorizer struct {
	opaURI        string
	cacher        *authzcache.OPACache
	restConfig    *rest.Config
	userExtraData *clusterinfo.ClusterInfo
}
type opaResponse struct {
	Result v1.SubjectAccessReview
}

func NewOPAAuthorizer(restConfig *rest.Config, opts *options.AuthorizerOptions) *OPAAuthorizer {
	ue, err := clusterinfo.FromUrl(opts.ExtrasPath, opts.ExtrasAnnotationPrefix)
	if err != nil {
		klog.Error(err.Error())
		ue = nil
	}
	return &OPAAuthorizer{restConfig: restConfig, opaURI: opts.AuthorizerUri, cacher: authzcache.NewOPACache(), userExtraData: ue}
}

func convertToV1Authz(clusterinfo map[string][]string) map[string]v1.ExtraValue {
	extraValues := map[string]v1.ExtraValue{}
	for k, v := range clusterinfo {
		extraValues[k] = v
	}
	return extraValues
}

func NewSubjectAccessReviewFromAttributes(attrs authorizer.Attributes) *v1.SubjectAccessReview {
	userExtraValues := convertToV1Authz(attrs.GetUser().GetExtra())
	// klog.Errorf("%v", attrs.GetUser().GetExtra())
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

// func (a *OPAAuthorizer) addClusterInfo(userExtra *map[string]v1.ExtraValue) {
// 	if a.userExtraData != nil {
// 		for k, v := range *a.userExtraData {
// 			(*userExtra)[k] = v
// 		}
// 	}
// }

func (a *OPAAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return a.authorize(ctx, attrs, authzRequestFunc(a.opaURI))
}

func (a *OPAAuthorizer) authorize(ctx context.Context, attrs authorizer.Attributes, authzFn func(*v1.SubjectAccessReview, *authzcache.OPACache) (*v1.SubjectAccessReview, error)) (authorizer.Decision, string, error) {
	sar := NewSubjectAccessReviewFromAttributes(attrs)
	// a.addClusterInfo(&sar.Spec.Extra)
	// request authorizer
	responseSAR, err := authzFn(sar, a.cacher)
	if responseSAR == nil || err != nil {
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
			cacheKey, err := createOpaRequestPayload(sar)
			if err == nil {
				if err = a.cacher.Put(string(cacheKey), cachePositive); err != nil {
					klog.Error(err.Error())
				}
			}
		} else {
			klog.Errorf("error marshaling SAR: %s", err.Error())
		}
	}
	return authorizer.DecisionAllow, responseSAR.Status.Reason, nil
}

func authzRequestFunc(uri string) func(*v1.SubjectAccessReview, *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
	return func(sar *v1.SubjectAccessReview, cache *authzcache.OPACache) (*v1.SubjectAccessReview, error) {
		var resp opaResponse
		jsonPayload, err := createOpaRequestPayload(sar)
		if err != nil {
			return nil, err
		}
		// check cache
		if cache != nil {
			bytes, ok := cache.Get(string(jsonPayload))
			if ok {
				cachedResponse := &v1.SubjectAccessReview{}
				err := json.Unmarshal(bytes, cachedResponse)
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

func createOpaRequestPayload(sar *v1.SubjectAccessReview) ([]byte, error) {
	sarSerializer := k8sJson.NewSerializerWithOptions(k8sJson.DefaultMetaFactory, nil, nil, k8sJson.SerializerOptions{
		Yaml:   false,
		Pretty: false,
		Strict: true,
	})
	var buf []byte
	serialisedSAR := bytes.NewBuffer(buf)
	err := sarSerializer.Encode(sar, serialisedSAR)
	if err != nil {
		return []byte{}, err
	}
	jsonPayload := fmt.Sprintf("{\"input\": %s}", serialisedSAR.Bytes())
	return []byte(jsonPayload), nil
}
