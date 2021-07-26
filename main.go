package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"admission-registry/pkg"

	"k8s.io/klog"
)

func main() {

	var param pkg.WhSvrParam
	// webhook http server (tls)
	// 命令行参数传递证书参数
	flag.IntVar(&param.Port, "port", 443, "webhook server tls port")
	flag.StringVar(&param.CertFile, "tleCertFile", "/etc/webhook/certs/tls.crt", "x509 certificate")
	flag.StringVar(&param.KeyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "x509 private key file")
	flag.Parse()

	// 加载证书文件生成证书对象
	certificate, err := tls.LoadX509KeyPair(param.CertFile, param.KeyFile)
	if err != nil {
		klog.Errorf("Failed to load key pair: %s", err)
		return
	}

	whSrv := pkg.WebhookServer{
		Server: &http.Server{
			// 配置端口
			Addr:    fmt.Sprintf(":%v", param.Port),
			Handler: nil,
			// 配置证书
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{certificate},
			},
		},
		// 获取白名单以逗号分割
		WhiteListRegistries: strings.Split(os.Getenv("WHITELIST_REGISTRIES"), ","),
	}

	// 定义http server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", whSrv.Handler)
	mux.HandleFunc("/mutate", whSrv.Handler) // 暂不实现

	// 注册handler
	whSrv.Server.Handler = mux

	// 使用新的go routine启动webhook server
	go func() {
		err := whSrv.Server.ListenAndServeTLS("", "")
		if err != nil {
			klog.Errorf("Failed to startup: %s", err)
		}
	}()

	klog.Info("Server started")

	// 监听os的关闭信号
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan

	klog.Info("Got OS shutdown signal, gracefully shutting down...")

	if err != whSrv.Server.Shutdown(context.Background()) {
		klog.Errorf("HTTP Server Shutdown error: %s", err)
	}

}
