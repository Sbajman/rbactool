package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	//"os"
	"path/filepath"
	"strings"
	"time"

	
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	//"k8s.io/client-go/rest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	httpSwagger "github.com/swaggo/http-swagger"
)

type RoleBindingRequest struct {
	Username  string `json:"username"`
	Namespace string `json:"namespace"`
	Duration  int    `json:"duration"`
}

var clientset *kubernetes.Clientset

// func init() {
// 	config, err := rest.InClusterConfig()
// 	if err != nil {
// 		log.Fatalf("Error creating in-cluster config: %v", err)
// 	}
// 	clientset, err = kubernetes.NewForConfig(config)
// 	if err != nil {
// 		log.Fatalf("Error creating clientset: %v", err)
// 	}
// }

func init() {
	// Load Kubernetes config from local kubeconfig
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error loading kubeconfig: %v", err)
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}
}


// // Handler to return list of namespaces
// func getNamespaces(w http.ResponseWriter, r *http.Request) {
// 	namespaceList, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to list namespaces: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	namespaces := []string{}
// 	for _, ns := range namespaceList.Items {
// 		namespaces = append(namespaces, ns.Name)
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string][]string{"namespaces": namespaces})
// }

func getNamespaces(w http.ResponseWriter, r *http.Request) {
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list namespaces: %v", err), http.StatusInternalServerError)
		return
	}

	var namespaceList []string
	for _, ns := range namespaces.Items {
		// Exclude any namespace ending with "-prod"
		if !strings.HasSuffix(ns.Name, "-prod") {
			namespaceList = append(namespaceList, ns.Name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(namespaceList)
}

func createRoleBinding(w http.ResponseWriter, r *http.Request) {
	var req RoleBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	expiration := time.Now().Add(time.Duration(req.Duration) * time.Hour)
	roleBindingName := fmt.Sprintf("rb-%d-%s", time.Now().Unix(), req.Username)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: req.Namespace,
			Annotations: map[string]string{
				"expiration": expiration.Format(time.RFC3339),
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: req.Username,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "view",
		},
	}

	_, err := clientset.RbacV1().RoleBindings(req.Namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create RoleBinding: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "RoleBinding created successfully"})
}

// cleanupExpiredRoleBindings checks and deletes expired rolebindings
func cleanupExpiredRoleBindings() {
	for {
		// Periodically check and delete expired RoleBindings
		time.Sleep(5 * time.Minute)
		log.Println("Checking for expired RoleBindings...")

		namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Failed to list namespaces: %v", err)
			continue
		}

		for _, ns := range namespaces.Items {
			roleBindings, err := clientset.RbacV1().RoleBindings(ns.Name).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				log.Printf("Failed to list RoleBindings in namespace %s: %v", ns.Name, err)
				continue
			}

			for _, rb := range roleBindings.Items {
				if expirationStr, exists := rb.Annotations["expiration"]; exists {
					expirationTime, err := time.Parse(time.RFC3339, expirationStr)
					if err != nil {
						log.Printf("Invalid expiration format for RoleBinding %s: %v", rb.Name, err)
						continue
					}
					if time.Now().After(expirationTime) {
						log.Printf("Deleting expired RoleBinding: %s in namespace %s", rb.Name, ns.Name)
						err := clientset.RbacV1().RoleBindings(ns.Name).Delete(context.TODO(), rb.Name, metav1.DeleteOptions{})
						if err != nil {
							log.Printf("Failed to delete RoleBinding %s: %v", rb.Name, err)
						}
					}
				}
			}
		}
	}
}

// func cleanupExpiredRoleBindingsHandler(w http.ResponseWriter, r *http.Request) {
//     go cleanupExpiredRoleBindings() // Run cleanup asynchronously
//     w.WriteHeader(http.StatusOK)
//     json.NewEncoder(w).Encode(map[string]string{"message": "Cleanup triggered successfully"})
// }

// cleanupExpiredRoleBindingsManual is a manual trigger endpoint to cleanup expired RoleBindings
func cleanupExpiredRoleBindingsManual(w http.ResponseWriter, r *http.Request) {
	// Triggering the cleanup process on request
	log.Println("Manually cleaning up expired RoleBindings...")
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list namespaces: %v", err), http.StatusInternalServerError)
		return
	}

	for _, ns := range namespaces.Items {
		roleBindings, err := clientset.RbacV1().RoleBindings(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Failed to list RoleBindings in namespace %s: %v", ns.Name, err)
			continue
		}

		for _, rb := range roleBindings.Items {
			if expirationStr, exists := rb.Annotations["expiration"]; exists {
				expirationTime, err := time.Parse(time.RFC3339, expirationStr)
				if err != nil {
					log.Printf("Invalid expiration format for RoleBinding %s: %v", rb.Name, err)
					continue
				}
				if time.Now().After(expirationTime) {
					log.Printf("Deleting expired RoleBinding: %s in namespace %s", rb.Name, ns.Name)
					err := clientset.RbacV1().RoleBindings(ns.Name).Delete(context.TODO(), rb.Name, metav1.DeleteOptions{})
					if err != nil {
						log.Printf("Failed to delete RoleBinding %s: %v", rb.Name, err)
					}
				}
			}
		}
	}

	// Send a response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Expired RoleBindings cleaned up successfully"})
}

func main() {
	http.HandleFunc("/namespaces", getNamespaces)
	http.HandleFunc("/create", createRoleBinding)
	http.HandleFunc("/cleanup", cleanupExpiredRoleBindingsManual) // New endpoint for manual cleanup
	http.Handle("/swagger/", httpSwagger.WrapHandler)
	go cleanupExpiredRoleBindings() // Keep periodic cleanup running in background
    http.Handle("/", http.FileServer(http.Dir("./static")))
	log.Println("Server running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
