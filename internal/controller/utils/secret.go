package utils

import (
	"context"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetSecret(clientset kubernetes.Interface, key, namespace string, name string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		zap.L().Error("get secret failed", zap.Error(err))
		return "", err
	}
	s := string(secret.Data[key])
	return s, nil
}
