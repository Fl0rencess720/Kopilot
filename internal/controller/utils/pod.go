package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	logsource "github.com/Fl0rencess720/Kopilot/pkg/logSource"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CheckPodHealthyStatus(status corev1.PodStatus) bool {
	if status.Phase != corev1.PodRunning && status.Phase != corev1.PodSucceeded {
		return false
	}

	if status.Phase == corev1.PodRunning {
		for _, containerStatus := range status.ContainerStatuses {
			if !containerStatus.Ready {
				return false
			}

			if containerStatus.State.Waiting != nil {
				reason := containerStatus.State.Waiting.Reason
				if reason == "CrashLoopBackOff" || reason == "Error" ||
					reason == "ImagePullBackOff" || reason == "ErrImagePull" {
					return false
				}
			}
			if containerStatus.State.Terminated != nil &&
				containerStatus.State.Terminated.ExitCode != 0 {
				return false
			}
		}

		for _, initStatus := range status.InitContainerStatuses {
			if initStatus.State.Waiting != nil {
				reason := initStatus.State.Waiting.Reason
				if reason == "CrashLoopBackOff" || reason == "Error" ||
					reason == "ImagePullBackOff" || reason == "ErrImagePull" {
					return false
				}
			}
		}
	}

	return true
}

func GetPodLogsFromKubernetes(clientset kubernetes.Interface, podName string, namespace string) (string, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod: %v", err)
	}

	hasRunningContainer := false
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil {
			hasRunningContainer = true
			break
		}
	}

	if !hasRunningContainer {
		return generateStatusMessage(pod.Status), nil
	}

	var tailLines int64 = 20
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{TailLines: &tailLines})

	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return fmt.Sprintf("Failed to retrieve logs: %v", err), nil
	}
	defer func() {
		if err := podLogs.Close(); err != nil {
			zap.L().Error("Error closing podLogs", zap.Error(err))
		}
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return fmt.Sprintf("Failed to read logs: %v", err), nil
	}

	return buf.String(), nil
}

func generateStatusMessage(status corev1.PodStatus) string {
	var messages []string

	messages = append(messages, fmt.Sprintf("Pod Phase: %s", status.Phase))

	for _, containerStatus := range status.ContainerStatuses {
		if containerStatus.State.Waiting != nil {
			messages = append(messages, fmt.Sprintf("Container %s: %s - %s",
				containerStatus.Name,
				containerStatus.State.Waiting.Reason,
				containerStatus.State.Waiting.Message))
		} else if containerStatus.State.Terminated != nil {
			messages = append(messages, fmt.Sprintf("Container %s: Terminated - %s",
				containerStatus.Name,
				containerStatus.State.Terminated.Reason))
		}
	}

	return strings.Join(messages, "\n")
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
