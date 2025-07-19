package utils

import (
	"bytes"
	"context"
	"io"

	logsource "github.com/Fl0rencess720/Kopilot/pkg/logSource"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func CheckPodHealthyStatus(status corev1.PodStatus) bool {
	if status.Phase != corev1.PodRunning && status.Phase != corev1.PodSucceeded {
		return false
	}
	return true
}

func GetPodLogsFromKubernetes(clientset kubernetes.Interface, podName string, namespace string) (string, error) {
	var tailLines int64 = 20
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{TailLines: &tailLines})

	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer func() {
		if err := podLogs.Close(); err != nil {
			zap.L().Error("Error closing podLogs", zap.Error(err))
		}
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	str := buf.String()
	return str, nil
}

func GetPodLogsFromLoki(podName string, namespace string, address string) (string, error) {
	loki := logsource.NewLokiClient(address)
	logs, err := loki.FetchLogs(logsource.LokiQueryOptions{
		PodName:   podName,
		Namespace: namespace,
		Limit:     20,
	})
	if err != nil {
		zap.L().Error("Failed to fetch logs from Loki", zap.Error(err))
		return "", err
	}
	return logs, nil
}
