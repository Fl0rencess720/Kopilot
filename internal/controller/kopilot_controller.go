/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	kopilotv1 "github.com/Fl0rencess720/Kopilot/api/v1"
	"github.com/Fl0rencess720/Kopilot/internal/controller/utils"
	"github.com/Fl0rencess720/Kopilot/pkg/llm"
	"github.com/Fl0rencess720/Kopilot/pkg/llm/multiagent"
	"github.com/Fl0rencess720/Kopilot/pkg/sink/feishusink"
	"github.com/go-logr/logr"
	"github.com/robfig/cron"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// KopilotReconciler reconciles a Kopilot object
type KopilotReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Clientset     kubernetes.Interface
	DynamicClient dynamic.Interface
}

type UnHealthyPod struct {
	Namespace string
	Name      string
	Log       string
}

// +kubebuilder:rbac:groups=kopilot.fl0rencess720,resources=kopilots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kopilot.fl0rencess720,resources=kopilots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kopilot.fl0rencess720,resources=kopilots/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KopilotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	var kopilot kopilotv1.Kopilot
	if err := r.Get(ctx, req.NamespacedName, &kopilot); err != nil {
		l.Error(err, "unable to fetch Kopilot")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	l.Info("Reconciling Kopilot", "name", kopilot.Name, "namespace", kopilot.Namespace)

	schedule, err := cron.ParseStandard(kopilot.Spec.Schedule)
	if err != nil {
		l.Error(err, "unable to parse schedule", "schedule", kopilot.Spec.Schedule)
		return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
	}

	now := time.Now()
	var lastCheckTime time.Time
	if kopilot.Status.LastCheckTime != nil {
		lastCheckTime = kopilot.Status.LastCheckTime.Time
	} else {
		lastCheckTime = kopilot.GetCreationTimestamp().Time
	}

	expectedNextCheckTime := schedule.Next(lastCheckTime)
	if now.Before(expectedNextCheckTime) {
		l.Info("Skipping check", "nextCheckTime", expectedNextCheckTime)
		return ctrl.Result{RequeueAfter: expectedNextCheckTime.Sub(now)}, nil
	}

	unhealthyPods := r.getUnhealthyPods(ctx, l, kopilot.Spec.LogSource)

	if err := r.sendUnhealthyPodsToLLM(ctx, l, unhealthyPods, kopilot.Spec.LLM, kopilot.Spec.Notification.Sinks, kopilot.Spec.KnowledgeBase); err != nil {
		zap.L().Error("failed to send unhealthy pods to LLM", zap.Error(err))
		return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
	}

	kopilot.Status.LastCheckTime = &metav1.Time{Time: now}
	if err := r.Status().Update(ctx, &kopilot); err != nil {
		l.Error(err, "failed to update Kopilot status")

	}

	nextCheckTime := schedule.Next(now)
	return ctrl.Result{RequeueAfter: nextCheckTime.Sub(now)}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KopilotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kopilotv1.Kopilot{}).
		Named("kopilot").
		Complete(r)
}

func (r *KopilotReconciler) getUnhealthyPods(ctx context.Context, l logr.Logger, logSource kopilotv1.LogSourceSpec) []UnHealthyPod {
	pods, err := r.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		l.Error(err, "unable to list pods")
		return nil
	}

	var unhealthyPods []UnHealthyPod
	for _, pod := range pods.Items {
		if pod.Kind == "Kopilot" {
			continue
		}
		// fmt.Println("pod", pod.Name, "namespace", pod.Namespace, "status", pod.Status.Phase, "condition", pod.Status.Conditions)
		if !utils.CheckPodHealthyStatus(pod.Status) {

			logs := ""
			switch logSource.Type {
			case "Kubernetes":
				logs, err = utils.GetPodLogsFromKubernetes(r.Clientset, pod.Name, pod.Namespace)
				if err != nil {
					l.Error(err, "unable to get pod logs from kubernetes, skipping", "pod", pod.Name, "namespace", pod.Namespace)
					logs = fmt.Sprintf("Failed to retrieve logs: %v", err)
				}
			case "Loki":
				logs, err = utils.GetPodLogsFromLoki(pod.Name, pod.Namespace, logSource.Loki.Address)
				if err != nil {
					l.Error(err, "unable to get pod logs from loki, skipping", "pod", pod.Name, "namespace", pod.Namespace)
					logs = fmt.Sprintf("Failed to retrieve logs: %v", err)
				}
			default:
				l.Error(fmt.Errorf("unknown log source type: %s", logSource.Type), "unable to get pod logs")
				continue
			}

			unhealthyPods = append(unhealthyPods, UnHealthyPod{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Log:       logs,
			})
		}
	}

	return unhealthyPods
}

func (r *KopilotReconciler) sendUnhealthyPodsToLLM(ctx context.Context, l logr.Logger, unhealthyPods []UnHealthyPod, llmSpec kopilotv1.LLMSpec, sinks []kopilotv1.NotificationSink, knowledgeBase *kopilotv1.KnowledgeBaseSpec) error {
	var err error

	for _, pod := range unhealthyPods {
		var c llm.LLMClient
		var retriever *llm.HybridRetriever
		if knowledgeBase != nil {
			retriever, err = llm.NewHybridRetriever(ctx, r.Clientset, *knowledgeBase)
			if err != nil {
				l.Error(err, "unable to create hybrid retriever")
				return err
			}
		}
		switch llmSpec.WorkingMode {
		case "single":
			c, err = llm.NewLLMClient(ctx, r.Clientset, llmSpec, retriever)
			if err != nil {
				l.Error(err, "unable to create LLM client")
				return err
			}

			result, err := c.Analyze(ctx, pod.Namespace, pod.Name, pod.Log)
			if err != nil {
				l.Error(err, "unable to analyze pod", "pod", pod.Name, "namespace", pod.Namespace)
				return err
			}

			sink, err := feishusink.NewFeishuSink(r.Clientset, *sinks[0].Feishu)
			if err != nil {
				l.Error(err, "unable to create feishu sink")
				return err
			}
			if err := sink.SendBotMessage(pod.Namespace, pod.Name, result); err != nil {
				l.Error(err, "unable to send result to feishu")
				return err
			}
		case "multi":
			ma, err := multiagent.NewLogMultiAgent(ctx, r.Clientset, r.DynamicClient, llmSpec, retriever, llmSpec.Language)
			if err != nil {
				l.Error(err, "unable to create multiagent")
				return err
			}
			if err := ma.Run(ctx, pod.Log); err != nil {
				l.Error(err, "unable to run multiagent")
				return err
			}
		}
	}
	return nil
}
