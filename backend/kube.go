package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/oklog/ulid/v2"
)

//var kubeClient *kubernetes.Clientset

// func init() {
// 	config, err := rest.InClusterConfig()
// 	if err != nil {
// 		panic(err)
// 	}
// 	kubeClient, err = kubernetes.NewForConfig(config)
// 	if err != nil {
// 		panic(err)
// 	}
// }

var kubeClient *kubernetes.Clientset

func init() {
	var config *rest.Config
	var err error

	// Try in-cluster config first
	config, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to local kubeconfig
		kubeconfig := filepath.Join(
			os.Getenv("HOME"), ".kube", "config",
		)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Unable to load kubeconfig: %v", err)
		}
	}

	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}
}

func CreateRoleBinding(c *gin.Context) {
	var req struct {
		Username  string  `json:"username"`
		Namespace string  `json:"namespace"`
		Duration  float64 `json:"duration"`
		Role      string  `json:"role"` // view/edit
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	if strings.HasSuffix(req.Namespace, "-prod") {
		c.JSON(400, gin.H{"error": "Access to prod namespaces is restricted"})
		return
	}

	createby := "rbactool"
	expiry := time.Now().Add(time.Duration(req.Duration * float64(time.Hour))).Format(time.RFC3339)
	id := ulid.Make().String()
	name := fmt.Sprintf("%s-%s", createby, strings.ToLower(req.Username))

	rb := &v1.RoleBinding{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"created-by": "rbac-tool",
			},
			Annotations: map[string]string{
				"ulid":   id,
				"expiry": expiry,
			},
		},
		Subjects: []v1.Subject{
			{
				Kind:     "User",
				Name:     req.Username,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		RoleRef: v1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     req.Role, // "view" or "edit"
		},
	}

	_, err := kubeClient.RbacV1().RoleBindings(req.Namespace).Create(context.Background(), rb, meta.CreateOptions{})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create RoleBinding: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "RoleBinding created", "ulid": id})
}

func ListNamespaces(c *gin.Context) {
	nsList, err := kubeClient.CoreV1().Namespaces().List(context.Background(), meta.ListOptions{})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	names := []string{}
	for _, ns := range nsList.Items {
		if !strings.HasSuffix(ns.Name, "-prod") {
			names = append(names, ns.Name)
		}
	}
	c.JSON(200, gin.H{"namespaces": names})
}

func CleanupExpiredBindings(c *gin.Context) {
	deleted := 0
	rbs, _ := kubeClient.RbacV1().RoleBindings("").List(context.Background(), meta.ListOptions{
		LabelSelector: "created-by=rbac-tool",
	})

	for _, rb := range rbs.Items {
		expiry := rb.Annotations["expiry"]
		expiryTime, _ := time.Parse(time.RFC3339, expiry)
		if time.Now().After(expiryTime) {
			err := kubeClient.RbacV1().RoleBindings(rb.Namespace).Delete(context.Background(), rb.Name, meta.DeleteOptions{})
			if err == nil {
				deleted++
			}
		}
	}
	c.JSON(200, gin.H{"deleted": deleted})
}

// ListNamespaces returns all namespaces in the cluster
func ListNamespacesauto() ([]string, error) {
	namespaces, err := kubeClient.CoreV1().Namespaces().List(context.Background(), meta.ListOptions{})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, ns := range namespaces.Items {
		names = append(names, ns.Name)
	}
	return names, nil
}

// AutoCleanupExpiredRoleBindings scans and deletes expired rolebindings created by rbac-tool
func AutoCleanupExpiredRoleBindings() {
	namespaces, err := ListNamespacesauto()
	if err != nil {
		log.Printf("Failed to list namespaces: %v", err)
		return
	}

	for _, ns := range namespaces {
		rbs, err := kubeClient.RbacV1().RoleBindings(ns).List(context.Background(), meta.ListOptions{
			LabelSelector: "created-by=rbac-tool",
		})
		if err != nil {
			log.Printf("Failed to list RoleBindings in namespace %s: %v", ns, err)
			continue
		}

		for _, rb := range rbs.Items {
			expiryStr := rb.Annotations["expiry"]
			if expiryStr == "" {
				continue
			}

			expiryTime, err := time.Parse(time.RFC3339, expiryStr)
			if err != nil {
				log.Printf("Failed to parse expiry time for %s/%s: %v", ns, rb.Name, err)
				continue
			}

			if time.Now().After(expiryTime) {
				err := kubeClient.RbacV1().RoleBindings(ns).Delete(context.Background(), rb.Name, meta.DeleteOptions{})
				if err != nil {
					log.Printf("Failed to delete expired RoleBinding %s/%s: %v", ns, rb.Name, err)
				} else {
					log.Printf("Deleted expired RoleBinding %s/%s", ns, rb.Name)
				}
			}
		}
	}
}
