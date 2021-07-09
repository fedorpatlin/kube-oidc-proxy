// Copyright Jetstack Ltd. See LICENSE for details.

package authorizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sJson "k8s.io/apimachinery/pkg/runtime/serializer/json"

	v1 "k8s.io/api/authorization/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog"
)

// Open Policy Agent authorizer
type OPAAuthorizer struct {
	opaURI string
}
type opaResponse struct {
	Result v1.SubjectAccessReview
}

func NewOPAAuthorizer(opts *options.AuthorizerOptions) *OPAAuthorizer {
	return &OPAAuthorizer{opaURI: opts.AuthorizerUri}
}
func (a *OPAAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	sar := v1.SubjectAccessReview{
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
			// Extra:  attrs.GetUser().GetExtra(),
		},
	}
	opaClient := http.Client{
		Timeout: 5 * time.Second,
	}
	jsonPayload := createOpaRequestPayload(&sar)
	// request authorizer
	// whauthz := webhook.WebhookAuthorizer{}
	authzResponse, err := opaClient.Post(a.opaURI, "application/json", bytes.NewReader([]byte(jsonPayload)))
	if err != nil {
		klog.Errorf("Authorization server is not responding: %s", err.Error())
	}
	defer authzResponse.Body.Close()
	if authzResponse.StatusCode == 200 {
		bodyBytes := make([]byte, 2048)
		bytesRead, err := authzResponse.Body.Read(bodyBytes)
		if err != nil && err != io.EOF {
			klog.Errorf("Error reading Authz response body: %s", err.Error())
			return authorizer.DecisionNoOpinion, "Authorizer not responding", fmt.Errorf("authorizer is not responding: %s", err.Error())
		}
		bodyBytes = bodyBytes[:bytesRead]
		var resp opaResponse
		err = json.Unmarshal(bodyBytes, &resp)
		if err != nil {
			klog.Errorf("Error reading authzResponse: %s", err.Error())
			klog.Error(string(bodyBytes))
			return authorizer.DecisionNoOpinion, "Authorizer response invalid", fmt.Errorf("error: %s", err.Error())
		}
		if resp.Result.Status.Denied {
			return authorizer.DecisionDeny, resp.Result.Status.Reason, nil
		}

		if !resp.Result.Status.Allowed {
			return authorizer.DecisionNoOpinion, resp.Result.Status.Reason, nil
		}
		return authorizer.DecisionAllow, "", nil
	}

	return authorizer.DecisionNoOpinion, "I have no idea about it", nil
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
