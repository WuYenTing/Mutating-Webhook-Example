# Kubernetes Mutating Webhook Server

The webhook automatically updates Kubernetes resources with specified annotations during the operation configure in mutatingwebhookconfiguration.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [How to test locally](#how-to-test-loaclly)
- [How to run in Kubernetes](#how-to-run-in-Kubernetes)
- [Deployment](#deployment)
- [How It Works](#how-it-works)
- [License](#license)

## Overview

The mutating webhook server processes incoming admission review requests from Kubernetes. It checks if the target resource (e.g., Service) requires mutations based on its kind, type, annotations, and namespace. If mutations are necessary, the server applies specified annotations to the resource.

## Prerequisites

- Go installed (version 1.17 or later)
- Kubernetes cluster (local or remote)
- kubectl installed for interacting with the Kubernetes cluster
- OpenSSL or Cert-manager to create TLS certificates

## Configuration

Before running the webhook server, configure the following parameters in your command line or environment:

- `-port`: Specify the port number for the webhook server (default is `443`).
- `-tlsCertFile`: Path to the x509 certificate file for HTTPS (default is `/webhook/certs/tls.crt`).
- `-tlsKeyFile`: Path to the private key file that matches the certificate (default is `/webhook/certs/tls.key`).
- `-kind`: Kind of the Kubernetes resource to mutate (default is `Service`).
- `-type`: Type of the resource (default is `LoadBalancer`).
- `-mutationKey`: The key to be added to annotations for resources (default is `service.beta.kubernetes.io/azure-load-balancer-internal`).
- `-mutationValue`: The value to be set for the mutation key (default is `true`).

## How to test locally

1. **Build the server**
   ```
   go build -o mutating-webhook
   ```
   or
   ```
   CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mutating-webhook
   ```    
    These arguments ensure that binaries are statically linked without dependencies on C libraries, enhancing compatibility and portability in a lightweight environment, and helps manage the build process effectively.

2. **Run the server**
   ```
   ./mutating-webhook -port=9443 -tlsCertFile=certs/tls.crt -tlsKeyFile=certs/key.crt
   ```

3. **Test with mock input**
   ```
   curl -X -POST -H "Content-Type: application/json" -d @inputs/annotation-not-exist.json https://127.0.0.1:9443/mutate -k
   ```
    -k argument for skipping SSL certificate verification which is not recommended to use in production.

4. **Result**

   **If the mutation does trigger**
   
   Server output
   ```
   APIVersion: admission.k8s.io/v1, Kind: AdmissionReview, UID: unique-request-id, Name: your-service-name, Namespace: your-namespace
   Ready to write reponse ...
   ```
   Client output
   ```
   {"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","response":{"uid":"unique-request-id","allowed":true,"patch":"W3sib3AiOiJhZGQiLCJwYXRoIjoiL21ldGFkYXRhL2Fubm90YXRpb25zIiwidmFsdWUiOnsic2VydmljZS5iZXRhLmt1YmVybmV0ZXMuaW8vYXp1cmUtbG9hZC1iYWxhbmNlci1pbnRlcm5hbCh0ZXN0KSI6InRydWUifX1d","patchType":"JSONPatch"}}  
   ```
   
   **If the mutation does not trigger because of the mutation key already exist**
   
   Server output
   ```
   Skip mutation for your-service-name since it alerady has the annotation key: "service.beta.kubernetes.io/azure-load-balancer-internal"
   Skipping validation for your-namespace/your-service-name due to policy check
   APIVersion: admission.k8s.io/v1, Kind: AdmissionReview, UID: unique-request-id, Name: your-service-name, Namespace: your-namespace
   Ready to write reponse ...
   ```
   Client output
   ```
   {"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","response":{"uid":"unique-request-id","allowed":true}}
   ```
   
   **If the mutation does not trigger because of unspport kind or type**
   
   Server output
   ```
   Skipping processing for unsupported type ClusterIP
   APIVersion: admission.k8s.io/v1, Kind: AdmissionReview, UID: unique-request-id, Name: your-service-name, Namespace: your-namespace
   Ready to write reponse ...
   ```
   Client output
   ```
   {"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","response":{"uid":"unique-request-id","allowed":true}}
   ```

## How to Run in Kubernetes

1. **Use Cert-manager to manage certifcate**

   a. cluster issuer (selfsigned-issuer) 
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: az-lb-in-anno-webhook-root-issuer
   spec:
     selfSigned: {}
   ```

   b. certificate (root-ca)
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: az-lb-in-anno-webhook-selfsigned-ca
   spec:
     commonName: az-lb-in-anno-webhook-selfsigned-ca
     isCA: true
     issuerRef:
       group: cert-manager.io
       kind: ClusterIssuer
       name: az-lb-in-anno-webhook-root-issuer
     privateKey:
       algorithm: ECDSA
       size: 256
     secretName: az-lb-in-anno-webhook-root-secret
   ```

   c. secret  
   ```yaml
   You will get the secret (az-lb-in-anno-webhook-root-secret) from the certificate (az-lb-in-anno-webhook-selfsigned-ca) above
   ```

   d. issuer  
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: az-lb-in-anno-webhook-issuer
     namespace: default
   spec:
     ca:
       secretName: az-lb-in-anno-webhook-root-secret
   ```

   e. certificate (mutating webhook ca)
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: az-lb-in-anno-webhook-cert
   spec:
     commonName: az-lb-in-anno-webhook.default.svc
     dnsNames:
       - az-lb-in-anno-webhook.default.svc
       - az-lb-in-anno-webhook.west.com
     issuerRef:
       group: cert-manager.io
       kind: Issuer
       name: az-lb-in-anno-webhook-issuer
     privateKey:
       algorithm: ECDSA
       size: 256
     secretName: az-lb-in-anno-webhook-cert-secret
     usages:
       - server auth
   ```
  
   f. secret
   ```yaml
   You will get the secret (az-lb-in-anno-webhook-cert-secret) from the certificate (az-lb-in-anno-webhook-cert) above
   ```

2. **Configure the mutating webhook**

   a. mutatingwebhookconfiguration
   ```yaml
   apiVersion: admissionregistration.k8s.io/v1
   kind: MutatingWebhookConfiguration
   metadata:
     name: az-lb-in-anno-webhook
     labels:
       app: az-lb-in-anno-webhook
     annotations:
       cert-manager.io/inject-ca-from: default/az-lb-in-anno-webhook-cert
   webhooks:
     - name: az-lb-in-anno-webhook.west.com
       clientConfig:
         service:
           name: az-lb-in-anno-webhook
           namespace: default
           path: "/mutate"
       rules:
         - operations: ["CREATE", "UPDATE"]
           apiGroups: [""]
           apiVersions: ["v1"]
           resources: ["services"]
       admissionReviewVersions: ["v1", "v1beta1"]
       sideEffects: None
       timeoutSeconds: 10
       failurePolicy: Ignore
   ```

   b. service 
   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: az-lb-in-anno-webhook
     labels:
       app: az-lb-in-anno-webhook
   spec:
     ports:
     - port: 443
       targetPort: 443
     selector:
       app: az-lb-in-anno-webhook
   ```

   c. serviceaccount
   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: az-lb-in-anno-webhook-sa
     labels:
       app: az-lb-in-anno-webhook
   ```

   d. deployment 
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: az-lb-in-anno-webhook
     labels:
       app: az-lb-in-anno-webhook
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: az-lb-in-anno-webhook
     template:
       metadata:
         labels:
           app: az-lb-in-anno-webhook
       spec:
         serviceAccount: az-lb-in-anno-webhook-sa
         volumes:
           - name: az-lb-in-anno-webhook-tls
             secret:
               secretName: az-lb-in-anno-webhook-cert-secret
         containers:
           - name: az-lb-in-anno-webhook
             image: iotplatformdev.azurecr.io/mutating-webhook:map #map version for key in OOO/OOO format can't directly add to patch operation path
             imagePullPolicy: Always
             args:
               - -tlsCertFile=/webhook/certs/tls.crt
               - -tlsKeyFile=/webhook/certs/tls.key
               - -kind=Service
               - -type=LoadBalancer
               - -mutationKey=service.beta.kubernetes.io/azure-load-balancer-internal
               - -mutationValue=true
             volumeMounts:
               - name: az-lb-in-anno-webhook-tls
                 mountPath: /webhook/certs
                 readOnly: true
   ```

## Learn more

- Webhook configuration:\
    https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/mutating-webhook-configuration-v1/
- Dynamic admission control:\
    https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#mutating-webhook-auditing-annotations
- Kubernetes-api for mutating webhook:\
    https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#mutatingwebhookconfiguration-v1-admissionregistration-k8s-io \
    https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#mutatingwebhook-v1-admissionregistration-k8s-io
- Cert manager:\
    https://cert-manager.io/docs/configuration/selfsigned/
- How to create a patch operation:\
    https://www.rfc-editor.org/rfc/rfc6902.html
