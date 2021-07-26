package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	AnnotationMutateKey  = "AnnotationMutateKey"
	AnnotationStatusKey  = "AnnotationStatusKey"
	SideCarContainerName = "nginx"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (s *WebhookServer) mutateAnnotations(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	// 取出admissionReview里面的Request
	req := ar.Request
	var (
		objectMeta        *metav1.ObjectMeta
		resourceNamespace string
		resourceName      string
		deployment        *appsv1.Deployment
		service           *corev1.Service
	)
	klog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v Operation=%v", req.Kind.Kind, req.Namespace, req.Name, req.UID, req.Operation)

	// switch Deployment & Service
	switch req.Kind.Kind {
	case "Deployment":
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			klog.Errorf("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = deployment.Name, deployment.Namespace, &deployment.ObjectMeta
	case "Service":
		if err := json.Unmarshal(req.Object.Raw, &service); err != nil {
			klog.Errorf("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = service.Name, service.Namespace, &service.ObjectMeta
	default:
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Can't handle this kind(%s) object", req.Kind.Kind),
			},
		}
	}
	if !mutationRequired(objectMeta) {
		// 如果不需要直接返回admissionResponse
		klog.Infof("Skipping validation for %s/%s due to policy check", resourceNamespace, resourceName)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}
	// 需要mutate则初始化Annotation
	annotations := map[string]string{AnnotationStatusKey: "mutated"}

	// patch annotation
	var patch []patchOperation

	patch = append(patch, updateAnnotation(objectMeta.GetAnnotations(), annotations)...)

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}
	klog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// mutationRequired 通过meta信息判断该资源是否需要mutate
func mutationRequired(metadata *metav1.ObjectMeta) bool {
	// 获取Annotation
	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	var required bool
	// lowerCase annotation target Key
	switch strings.ToLower(annotations[AnnotationMutateKey]) {
	default:
		required = true
	case "n", "no", "false", "off":
		required = false
	}
	status := annotations[AnnotationStatusKey]

	if strings.ToLower(status) == "mutated" {
		required = false
	}

	klog.Infof("Mutation policy for %v/%v: required:%v", metadata.Namespace, metadata.Name, required)
	return required
}

// updateAnnotation 返回一个Annotation的Patch操作
func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}

func (s *WebhookServer) mutateContainers(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	// 取出admissionReview里面的Request
	req := ar.Request
	var (
		objectMeta        *metav1.ObjectMeta
		resourceNamespace string
		resourceName      string
		deployment        *appsv1.Deployment
	)
	klog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v Operation=%v", req.Kind.Kind, req.Namespace, req.Name, req.UID, req.Operation)

	if req.Namespace != corev1.NamespaceDefault {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Only Default namespace supported: %s", req.Namespace),
			},
		}
	}

	// switch Deployment & Service
	switch req.Kind.Kind {

	case "Deployment":
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			klog.Errorf("Could not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = deployment.Name, deployment.Namespace, &deployment.ObjectMeta
	default:
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Skip handle this kind(%s) object bacause handle Deployment Only", req.Kind.Kind),
			},
		}
	}

	if !mutationRequired(objectMeta) {
		// 如果不需要直接返回admissionResponse
		klog.Infof("Skipping validation for %s/%s due to policy check", resourceNamespace, resourceName)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}
	// patch annotation
	var patch []patchOperation

	patch = append(patch, updateDeploymentSpec(deployment.Spec)...)

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}
	klog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// updateDeploymentSpec 添加SideCar容器
func updateDeploymentSpec(deploySpec appsv1.DeploymentSpec) (patch []patchOperation) {
	for _, obj := range deploySpec.Template.Spec.Containers {
		if obj.Name == SideCarContainerName {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/spec/template/spec/containers/" + obj.Name,
				Value: obj,
			})
		} else {
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/spec/template/spec/containers",
				Value: []corev1.Container{
					{
						Name:    SideCarContainerName,
						Image:   "nginx:1.18.0",
						Command: []string{"/bin/bash", "-c", "sleep", "6000"},
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: 80,
								Protocol:      "TCP",
							},
						},
					},
				},
			})
		}
	}
	return patch
}
