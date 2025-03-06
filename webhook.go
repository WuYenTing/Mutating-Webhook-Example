package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	defaulter     = runtime.ObjectDefaulter(runtimeScheme)
)

var (
	targetKind string
	targetType string
	mutationKey string
	mutationValue string
	addAnnotations = map[string]string{
	}
	ignoredAnnotationkey string
	ignoredNamespaces = []string{
		metav1.NamespacePublic,
		metav1.NamespaceSystem,
	}
)

type WebhookServer struct {
	server *http.Server
}

type WebhookServerParameters struct {
	port              int               // webhook server port
	certFile          string            // path to the x509 certificate for https
	keyFile           string            // path to the x509 private key matching `CertFile`
	sidecarCfgFile    string            // path to sidecar injector configuration file
}

type PatchOperation struct {
	Op                string            `json:"op"`
	Path              string            `json:"path"`
	Value             interface{}       `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1.AddToScheme(runtimeScheme)
}

// Check if the resource needs mutation
func mutationRequired(ignoredList []string, metadata *metav1.ObjectMeta, ignoredAnnotationKey string) bool {
	required := true

	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			fmt.Printf("Skip mutation for %v since it's in special namespace: %v\n", metadata.Name, metadata.Namespace)
			required = false
		}
	}

	annotations := metadata.GetAnnotations()
	_, exist := annotations[ignoredAnnotationkey]
	if exist == true {
		fmt.Printf("Skip mutation for %v since it alerady has the annotation key: \"%v\"\n", metadata.Name, ignoredAnnotationKey)
		required = false
	}

	return required
}

// Create the patch operation
func createPatch(availableAnnotations map[string]string, addannotations map[string]string) ([]byte, error) {
	if availableAnnotations == nil {
		availableAnnotations = map[string]string{}
	}
	var patch []PatchOperation
	for key, value := range addannotations {
		if availableAnnotations[key] == "" {
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/metadata/annotations/" + key,
				Value: value,
				// map[string]string method below is for annotation key string with slash ex. XXXX/XXXX
				// Op:    "add",
				// Path:  "/metadata/annotations",
				// Value: map[string]string{key: value}, 
			})
		}
	}

	return json.Marshal(patch)
}

// Define the mutate policy here
func (whsvr *WebhookServer) mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	var (
		service                               corev1.Service
		availableAnnotations                  map[string]string
		objectMeta                            *metav1.ObjectMeta
		resourceNamespace, resourceName       string
	)

 	if req.Kind.Kind == targetKind {
		err := json.Unmarshal(req.Object.Raw, &service)
		if err != nil {
			fmt.Printf("Could not unmarshal raw object: %v\n", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}

		resourceName, resourceNamespace, objectMeta = service.Name, service.Namespace, &service.ObjectMeta
		availableAnnotations = service.Annotations
		
		if service.Spec.Type != corev1.ServiceType(targetType) {
			fmt.Printf("Skipping processing for unsupported type %v\n", service.Spec.Type)
				return &admissionv1.AdmissionResponse{
					Allowed: true,
			}
		}

	} else {
		fmt.Printf("Skipping processing for unsupported kind %v\n", req.Kind.Kind)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	if !mutationRequired(ignoredNamespaces, objectMeta, ignoredAnnotationkey) {
		fmt.Printf("Skipping validation for %v/%v due to policy check\n", resourceNamespace, resourceName)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	patchBytes, err := createPatch(availableAnnotations, addAnnotations)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		fmt.Printf("empty body\n")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		fmt.Printf("Content-Type=%v, expect application/json\n", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		fmt.Printf("Can't decode body: %v\n", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		if r.URL.Path == "/mutate" {
			admissionResponse = whsvr.mutate(&ar)
		}
	}

	admissionReview := admissionv1.AdmissionReview{}
	admissionReview.APIVersion = ar.APIVersion
	admissionReview.Kind = ar.Kind
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		fmt.Printf("Can't encode response: %v\n", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	fmt.Printf("APIVersion: %v, Kind: %v, UID: %v, Name: %v, Namespace: %v\n", ar.APIVersion, ar.Kind, ar.Request.UID, ar.Request.Name, ar.Request.Namespace)
	fmt.Printf("Ready to write reponse ...\n")
	_, err = w.Write(resp)
	if err != nil {
		fmt.Printf("Can't write response: %v\n", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
