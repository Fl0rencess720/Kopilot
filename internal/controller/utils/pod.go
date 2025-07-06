package utils

import (
	"bytes"
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func CheckPodHealthyStatus(status corev1.PodStatus) bool {
	if status.Phase != corev1.PodRunning && status.Phase != corev1.PodSucceeded {
		return false
	}
	return true
}

func GetPodLogs(clientset kubernetes.Interface, podName string, namespace string) (string, error) {
	var tailLines int64 = 20
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{TailLines: &tailLines})

	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	str := buf.String()
	return str, nil
}
