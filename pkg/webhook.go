package pkg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	corev1 "k8s.io/api/core/v1"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
)

var (
	// 生成格式化请求格式工厂
	runtimeScheme = runtime.NewScheme()
	codeFactory   = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codeFactory.UniversalDeserializer()
)

// WhSvrParam 定义webhook server参数的结构体
type WhSvrParam struct {
	Port           int
	CertFile       string
	KeyFile        string
	SidecarCfgFile string
}

type WebhookServer struct {
	SidecarConfig       *Config
	Server              *http.Server // http server
	WhiteListRegistries []string     // 白名单镜像仓库列表
}

type Config struct {
	Containers []corev1.Container `yaml:"containers"`
	Volumes    []corev1.Volume    `yaml:"volumes"`
}

func (s *WebhookServer) Handler(writer http.ResponseWriter, request *http.Request) {
	var body []byte
	if request.Body != nil {
		data, err := ioutil.ReadAll(request.Body)
		if err != nil {
			klog.Errorf("Validating request reading body error: %s", err)
			http.Error(writer, fmt.Sprintf("Validating request reading body error: %s", err), http.StatusBadRequest)
			return
		}
		body = data
	}
	if len(body) == 0 {
		klog.Error("Empty body data")
		http.Error(writer, "Empty data body", http.StatusBadRequest)
		return
	}

	// 校验 content-type
	contentType := request.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type is %s, but expected application/json", contentType)
		http.Error(writer, fmt.Sprintf("Content-Type is %s, but expected application/json", contentType), http.StatusBadRequest)
	}

	// 数据序列化
	requestedAdmissionReview := admissionv1.AdmissionReview{}
	var admissionResponse *admissionv1.AdmissionResponse

	//  返回值暂不做处理
	if _, _, err := deserializer.Decode(body, nil, &requestedAdmissionReview); err != nil {
		klog.Errorf("Cant decode body: %v", err)
		// 构造admissionResponse
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			},
		}
	} else {
		// 序列化成功 也就是获取到了请求admission review的数据
		if request.URL.Path == "/mutate" {
			// TODO
			//admissionResponse = s.mutateAnnotations(&requestedAdmissionReview)
			admissionResponse = s.mutate(&requestedAdmissionReview)

		} else if request.URL.Path == "/validate" {
			admissionResponse = s.validate(&requestedAdmissionReview)
		}
	}

	// 构造返回的 AdmissionReview 这个结构体
	responseAdmissionReview := admissionv1.AdmissionReview{}

	// 根据请求来的requestedAdmissionReview.APIVersion 配置 responseAdmissionReview.APIVersion
	responseAdmissionReview.APIVersion = requestedAdmissionReview.APIVersion
	responseAdmissionReview.Kind = requestedAdmissionReview.Kind

	if admissionResponse != nil {
		responseAdmissionReview.Response = admissionResponse
		if requestedAdmissionReview.Request != nil {
			// 返回相同的uid表示为同一请求
			responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		}
	}
	klog.Info(fmt.Sprintf("sending response: %s", responseAdmissionReview.Response))

	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		klog.Errorf("Can't encode response: %s", err)
		http.Error(writer, fmt.Sprintf("Can't encode response: %s", err), http.StatusInternalServerError)
	}
	klog.Info("Ready to write response")

	if _, err := writer.Write(respBytes); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(writer, fmt.Sprintf("Can't write response: %v", err), http.StatusBadRequest)
	}
}
