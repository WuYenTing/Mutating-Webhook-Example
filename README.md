# Kubernetes Mutating Webhook Server

The webhook automatically updates Kubernetes resources with specified annotations `annotation-mutation-webhook-example=true` during the operation configure in mutatingwebhookconfiguration.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [Test](#test)
- [Deploy](#deploy)
- [Learn more](#learn-more)

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
- `-mutationKey`: The key to be added to annotations for resources (default is `annotation-mutation-webhook-example`).
- `-mutationValue`: The value to be set for the mutation key (default is `true`).

## Test

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
   APIVersion: admission.k8s.io/v1, Kind: AdmissionReview, UID: XXXXXXXXXXXXX, Name: example-service,Namespace: example-namespace
   Ready to write reponse ...
   ```
   Client output
   ```
   {"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","response":{"uid":"XXXXXXXXXXXXX","allowed":true,"patch":"W3sib3AiOiJhZGQiLCJwYXRoIjoiL21ldGFkYXRhL2Fubm90YXRpb25zIiwidmFsdWUiOnsiYW5ub3RhdGlvbi1tdXRhdGlvbi13ZWJob29rLWV4YW1wbGUiOiJ0cnVlIn19XQ==","patchType":"JSONPatch"}}
   ```
   
   **If the mutation does not trigger because of the mutation key already exist**
   
   Server output
   ```
   Skip mutation for example-name since it alerady has the annotation key: "annotation-mutation-webhook-example"
   Skipping validation for example-namespace/example-name due to policy check
   APIVersion: admission.k8s.io/v1, Kind: AdmissionReview, UID: XXXXXXXXXXXXX, Name: example-service, Namespace: example-namespace
   Ready to write reponse ...
   ```
   Client output
   ```
   {"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","response":{"uid":"XXXXXXXXXXXXX","allowed":true}}
   ```
   
   **If the mutation does not trigger because of unspport kind or type**
   
   Server output
   ```
   Skipping processing for unsupported type ClusterIP
   APIVersion: admission.k8s.io/v1, Kind: AdmissionReview, UID: XXXXXXXXXXXXX, Name: example-service, Namespace: example-namespace
   Ready to write reponse ...
   ```
   Client output
   ```
   {"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","response":{"uid":"XXXXXXXXXXXXX","allowed":true}}
   ```

## Deploy

1. **Use Cert-manager to manage certifcate**

   a. cluster issuer (selfsigned-issuer) 
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: annotation-mutating-webhook-example-root-issuer
   spec:
     selfSigned: {}
   ```

   b. certificate (root-ca)
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: annotation-mutating-webhook-example-selfsigned-ca
   spec:
     commonName: annotation-mutating-webhook-example-selfsigned-ca
     isCA: true
     issuerRef:
       group: cert-manager.io
       kind: ClusterIssuer
       name: annotation-mutating-webhook-example-root-issuer
     privateKey:
       algorithm: ECDSA
       size: 256
     secretName: annotation-mutating-webhook-example-root-secret
   ```

   c. secret  
   ```yaml
   You will get the secret (annotation-mutating-webhook-example-root-secret) from the certificate (annotation-mutating-webhook-example-selfsigned-ca) above
   ```

   d. issuer  
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: annotation-mutating-webhook-example-issuer
     namespace: default
   spec:
     ca:
       secretName: annotation-mutating-webhook-example-root-secret
   ```

   e. certificate (mutating webhook ca)
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: annotation-mutating-webhook-example-cert
   spec:
     commonName: annotation-mutating-webhook-example.default.svc
     dnsNames:
       - annotation-mutating-webhook-example.default.svc
       - annotation-mutating-webhook-example.example.com
     issuerRef:
       group: cert-manager.io
       kind: Issuer
       name: annotation-mutating-webhook-example-issuer
     privateKey:
       algorithm: ECDSA
       size: 256
     secretName: annotation-mutating-webhook-example-cert-secret
     usages:
       - server auth
   ```
  
   f. secret
   ```yaml
   You will get the secret (annotation-mutating-webhook-example-cert-secret) from the certificate (annotation-mutating-webhook-example-cert) above
   ```

2. **Configure the mutating webhook**

   a. mutatingwebhookconfiguration
   ```yaml
   apiVersion: admissionregistration.k8s.io/v1
   kind: MutatingWebhookConfiguration
   metadata:
     name: annotation-mutating-webhook-example
     labels:
       app: annotation-mutating-webhook-example
     annotations:
       cert-manager.io/inject-ca-from: default/annotation-mutating-webhook-example-cert
   webhooks:
     - name: annotation-mutating-webhook-example.example.com
       clientConfig:
         service:
           name: annotation-mutating-webhook-example
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
     name: annotation-mutating-webhook-example
     labels:
       app: annotation-mutating-webhook-example
   spec:
     ports:
     - port: 443
       targetPort: 443
     selector:
       app: annotation-mutating-webhook-example
   ```

   c. serviceaccount
   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: annotation-mutating-webhook-example-sa
     labels:
       app: annotation-mutating-webhook-example
   ```

   d. deployment 
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: annotation-mutating-webhook-example
     labels:
       app: annotation-mutating-webhook-example
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: annotation-mutating-webhook-example
     template:
       metadata:
         labels:
           app: annotation-mutating-webhook-example
       spec:
         serviceAccount: annotation-mutating-webhook-example-sa
         volumes:
           - name: annotation-mutating-webhook-example-tls
             secret:
               secretName: annotation-mutating-webhook-example-cert-secret
         containers:
           - name: annotation-mutating-webhook-example
             image: ap95071/annotation-mutating-webhook-example:latest  #change the tag latest to map for annotation key string with slash ex. XXXX/XXXX
             imagePullPolicy: Always
             args:
               - -tlsCertFile=/webhook/certs/tls.crt
               - -tlsKeyFile=/webhook/certs/tls.key
               - -kind=Service
               - -type=LoadBalancer
               - -mutationKey=annotation-mutation-webhook-example
               - -mutationValue=true
             volumeMounts:
               - name: annotation-mutating-webhook-example-tls
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
