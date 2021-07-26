package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (s *WebhookServer) validate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	// TODO
	req := ar.Request
	var (
		allowed = true
		code    = 200
		message = ""
	)
	klog.Infof("AdmissionReview for kind=%s, Namespace=%s, Name=%s, UID=%s", req.Kind, req.Namespace, req.Name, req.UID)

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		klog.Errorf("Can't unmarshal object raw: %v", err)
		allowed = false
		code = http.StatusBadRequest
		message = err.Error()
		return &admissionv1.AdmissionResponse{
			Allowed: allowed,
			Result: &metav1.Status{
				Code:    int32(code),
				Message: message,
			},
		}
	}

	// 处理真正的业务逻辑
	for _, container := range pod.Spec.Containers {
		var whileListed = false
		for _, reg := range s.WhiteListRegistries {
			if strings.HasPrefix(container.Image, reg) {
				whileListed = true
			}
		}
		if !whileListed {
			allowed = false
			code = http.StatusForbidden
			message = fmt.Sprintf("%s image comes from an untrusted registry, only images from %v allowed", container.Image, s.WhiteListRegistries)
			break
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: allowed,
		Result: &metav1.Status{
			Code:    http.StatusOK,
			Message: message,
		},
	}
}
