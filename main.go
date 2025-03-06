package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var WhSvrParameters WebhookServerParameters
	
	flag.IntVar(&WhSvrParameters.port, "port", 443, "Webhook server port.")
	flag.StringVar(&WhSvrParameters.certFile, "tlsCertFile", "/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&WhSvrParameters.keyFile, "tlsKeyFile", "/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&targetKind, "kind", "Service", "Target Kind for the mutating webhook.")
	flag.StringVar(&targetType, "type", "LoadBalancer", "Target type for the mutating webhook.")
	flag.StringVar(&mutationKey, "mutationKey", "annotation-mutation-webhook-example", "Mutation Key for the mutating webhook.")
	flag.StringVar(&mutationValue, "mutationValue", "true", "Mutation value for the mutating webhook.")
	flag.Parse()
	addAnnotations[mutationKey]=mutationValue
	ignoredAnnotationkey=mutationKey

	fmt.Printf("Port: %v\n", WhSvrParameters.port)
	fmt.Printf("tlsCertFile path: %v\n", WhSvrParameters.certFile)
	fmt.Printf("tlsKeyFile path: %v\n", WhSvrParameters.keyFile)
	fmt.Printf("targetKind: %v\n", targetKind)
	fmt.Printf("targetType: %v\n", targetType)
	fmt.Printf("mutation %v\n", addAnnotations)


	CertKeyPair, err := tls.LoadX509KeyPair(WhSvrParameters.certFile, WhSvrParameters.keyFile)
	if err != nil {
		fmt.Printf("Failed to load key pair: %v\n", err)
	}

	Whsvr := &WebhookServer{
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", WhSvrParameters.port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{CertKeyPair},
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", Whsvr.serve)
	Whsvr.server.Handler = mux

	go func(){
		err := Whsvr.server.ListenAndServeTLS(WhSvrParameters.certFile, WhSvrParameters.keyFile)
		if err != nil {
			fmt.Printf("Failed to listen and serve webhook server: %v\n", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	fmt.Printf("Got OS shutdown signal, shutting down webhook server gracefully...\n")
	Whsvr.server.Shutdown(context.Background())
}
