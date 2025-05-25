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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	syncv1 "github.com/stangj/secretsync-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretsyncReconciler 结构体负责调和 Secretsync 自定义资源对象
// 实现了 controller-runtime 的 Reconciler 接口
type SecretsyncReconciler struct {
	client.Client                 // Kubernetes API 客户端接口
	Scheme        *runtime.Scheme // 用于序列化/反序列化对象以及设置所有者引用
	Log           logr.Logger     // 结构化日志接口
}

// 以下是控制器所需的 RBAC 权限注解
// +kubebuilder:rbac:groups=sync.example.com,resources=secretsyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sync.example.com,resources=secretsyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// 定义 Prometheus 指标变量，用于监控控制器性能和状态
var (
	// syncTotalCounter 记录同步操作的总次数及结果
	syncTotalCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "secretsync_total",
			Help: "Total number of sync operations",
		},
		[]string{"result"}, // 标签，用于区分成功/失败结果
	)

	// syncLatencySeconds 记录同步操作的延迟时间
	syncLatencySeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "secretsync_latency_seconds",
			Help: "Time taken for sync",
		},
	)

	// lastSuccessTimeGauge 记录最后一次成功同步的时间戳
	lastSuccessTimeGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "secretsync_last_success_time",
			Help: "Timestamp of last successful sync",
		},
	)
)

// init 函数在包加载时执行，注册 Prometheus 指标
func init() {
	prometheus.MustRegister(syncTotalCounter, syncLatencySeconds, lastSuccessTimeGauge)
}

// Reconcile 是控制器的核心方法，实现了 controller-runtime 的 Reconciler 接口
// 当 Secretsync 资源或相关资源发生变化时被调用
func (r *SecretsyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// 记录开始时间，用于计算延迟
	start := time.Now()
	// 创建带有请求信息的日志记录器
	log := r.Log.WithValues("secretsync", req.NamespacedName)

	// 获取 SecretSync 自定义资源对象
	var syncObj syncv1.Secretsync
	if err := r.Get(ctx, req.NamespacedName, &syncObj); err != nil {
		if errors.IsNotFound(err) {
			// 如果对象不存在，可能是已被删除，记录日志并退出
			log.Info("SecretSync CR not found, skip reconciliation")
			syncTotalCounter.WithLabelValues("failure").Inc()
			return ctrl.Result{}, nil
		}
		// 其他获取错误，记录失败并返回错误以触发重试
		log.Error(err, "Failed to get SecretSync CR")
		syncTotalCounter.WithLabelValues("failure").Inc()
		return ctrl.Result{}, err
	}

	// 验证必要的 spec 字段是否存在
	if syncObj.Spec.SourceNamespace == "" || syncObj.Spec.SourceSecretName == "" {
		log.Error(nil, "Invalid spec: SourceNamespace or SourceSecretName missing", "spec", syncObj.Spec)
		// 更新最后同步时间，即使同步失败
		syncObj.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
		_ = r.Status().Update(ctx, &syncObj)
		syncTotalCounter.WithLabelValues("failure").Inc()
		return ctrl.Result{}, nil
	}

	// 获取源 Secret 对象
	srcNamespaceName := types.NamespacedName{
		Namespace: syncObj.Spec.SourceNamespace,
		Name:      syncObj.Spec.SourceSecretName,
	}
	var srcSecret corev1.Secret
	if err := r.Get(ctx, srcNamespaceName, &srcSecret); err != nil {
		// 源 Secret 可能不存在或获取出错
		log.Error(err, "Failed to get source Secret", "secret", srcNamespaceName)
		syncTotalCounter.WithLabelValues("failure").Inc()
		return ctrl.Result{}, err
	}

	// 根据标签选择器获取匹配的目标命名空间列表
	namespaces, err := r.getMatchingNamespaces(ctx, syncObj.Spec.TargetNamespaceSelector)
	if err != nil {
		log.Error(err, "Failed to list matched namespaces")
		syncTotalCounter.WithLabelValues("failure").Inc()
		return ctrl.Result{}, err
	}

	// 确定目标 Secret 的名称
	// 如果没有指定，则使用源 Secret 的名称
	targetSecretName := syncObj.Spec.TargetSecretName
	if targetSecretName == "" {
		targetSecretName = srcSecret.Name
	}

	// 用于记录同步结果的数组
	var synced []string // 成功同步的命名空间
	var failed []string // 失败的命名空间

	// 遍历所有目标命名空间，执行同步
	for _, ns := range namespaces {
		if err := r.syncSecret(ctx, &srcSecret, ns, targetSecretName, &syncObj); err != nil {
			// 同步到当前命名空间失败，记录错误
			log.Error(err, "Failed to sync secret to namespace", "namespace", ns)
			failed = append(failed, ns)
		} else {
			// 同步成功，记录成功的命名空间
			synced = append(synced, ns)
		}
	}

	// 更新 Secretsync 资源的状态
	now := metav1.Now()
	syncObj.Status.SyncedNamespaces = synced
	syncObj.Status.FailedNamespaces = failed
	syncObj.Status.LastSyncTime = &now
	if err := r.Status().Update(ctx, &syncObj); err != nil {
		log.Error(err, "Failed to update SecretSync status")
		// 状态更新失败不会影响同步操作的返回结果
	}

	// 更新 Prometheus 指标
	if len(synced) > 0 && len(failed) == 0 {
		// 全部同步成功
		syncTotalCounter.WithLabelValues("success").Inc()
		lastSuccessTimeGauge.Set(float64(now.Unix()))
	} else if len(failed) > 0 {
		// 有失败的命名空间
		syncTotalCounter.WithLabelValues("failure").Inc()
	} else {
		// 部分成功部分失败
		syncTotalCounter.WithLabelValues("partial_success").Inc()
	}

	// 计算并记录同步操作的延迟
	latency := time.Since(start).Seconds()
	syncLatencySeconds.Observe(latency)

	// 如果有任何命名空间同步失败，返回错误以触发重新排队
	if len(failed) > 0 {
		return ctrl.Result{}, fmt.Errorf("some target namespaces failed to sync")
	}

	// 所有同步都成功，返回成功结果
	return ctrl.Result{}, nil
}

// syncSecret 将单个源 Secret 同步到目标命名空间中
// 参数:
// - ctx: 上下文，用于API通信
// - src: 源 Secret 对象
// - namespace: 目标命名空间
// - targetSecretName: 在目标命名空间中创建的 Secret 名称
// - syncObj: Secretsync 对象，用于设置所有者引用
func (r *SecretsyncReconciler) syncSecret(
	ctx context.Context,
	src *corev1.Secret,
	namespace, targetSecretName string,
	syncObj *syncv1.Secretsync,
) error {
	// 创建目标 Secret 对象
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetSecretName,
			Namespace: namespace,
		},
		Data: src.Data, // 复制源 Secret 的数据
		Type: src.Type, // 复制源 Secret 的类型
	}

	// 设置控制器引用，使 Secret 成为 Secretsync 的子资源
	// 注意：跨命名空间的所有者引用可能会导致问题
	controllerutil.SetControllerReference(syncObj, target, r.Scheme)

	// 检查目标 Secret 是否已存在
	var existing corev1.Secret
	existingKey := types.NamespacedName{Namespace: namespace, Name: targetSecretName}
	err := r.Get(ctx, existingKey, &existing)

	if err != nil {
		if errors.IsNotFound(err) {
			// Secret 不存在，创建新的
			return r.Create(ctx, target)
		}
		return err
	}

	// Secret 已存在，检查是否需要更新
	// 只有当数据或类型发生变化时才更新
	if !reflect.DeepEqual(existing.Data, target.Data) || existing.Type != target.Type {
		existing.Data = target.Data
		existing.Type = target.Type
		return r.Update(ctx, &existing)
	}

	// 数据和类型没有变化，无需更新
	return nil
}

// getMatchingNamespaces 根据标签选择器获取匹配的命名空间列表
// 参数:
// - ctx: 上下文，用于API通信
// - selector: Kubernetes 标签选择器
// 返回:
// - 匹配的命名空间名称列表
// - 错误（如果有）
func (r *SecretsyncReconciler) getMatchingNamespaces(
	ctx context.Context,
	selector *metav1.LabelSelector,
) ([]string, error) {
	// 如果没有提供选择器，返回空列表
	if selector == nil {
		return nil, nil
	}

	// 将 LabelSelector 转换为 Selector 接口
	selectorLabels, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}

	// 列出所有匹配选择器的命名空间
	var nsList corev1.NamespaceList
	if err := r.List(ctx, &nsList, client.MatchingLabelsSelector{Selector: selectorLabels}); err != nil {
		return nil, err
	}

	// 提取命名空间名称
	var namespaces []string
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

// enqueueSecrets 是一个 MapFunc，当监视的 Secret 发生变化时
// 确定哪些 Secretsync 对象需要被重新调和
// 返回需要重新调和的 Secretsync 请求列表
func (r *SecretsyncReconciler) enqueueSecrets(_ context.Context, obj client.Object) []reconcile.Request {
	ctx := context.TODO()
	// 转换为 Secret 对象
	secret := obj.(*corev1.Secret)

	// 列出所有 Secretsync 对象
	var list syncv1.SecretsyncList
	if err := r.List(ctx, &list, &client.ListOptions{}); err != nil {
		r.Log.Error(err, "Failed to list SecretSync CRs")
		return nil
	}

	// 查找使用此 Secret 作为源的所有 Secretsync 对象
	var requests []reconcile.Request
	for _, item := range list.Items {
		// 检查 Secret 是否是该 Secretsync 的源
		if item.Spec.SourceNamespace == secret.Namespace && item.Spec.SourceSecretName == secret.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&item),
			})
		}
	}
	return requests
}

// enqueueNamespaces 是一个 MapFunc，当监视的 Namespace 发生变化时
// 确定哪些 Secretsync 对象需要被重新调和
// 返回需要重新调和的 Secretsync 请求列表
func (r *SecretsyncReconciler) enqueueNamespaces(_ context.Context, obj client.Object) []reconcile.Request {
	ctx := context.TODO()
	// 转换为 Namespace 对象
	ns := obj.(*corev1.Namespace)

	// 列出所有 Secretsync 对象
	var list syncv1.SecretsyncList
	if err := r.List(ctx, &list, &client.ListOptions{}); err != nil {
		r.Log.Error(err, "Failed to list SecretSync CRs")
		return nil
	}

	// 查找所有可能使用此命名空间作为目标的 Secretsync 对象
	var requests []reconcile.Request
	for _, item := range list.Items {
		// 跳过没有目标命名空间选择器的 Secretsync
		if item.Spec.TargetNamespaceSelector == nil {
			continue
		}

		// 检查命名空间是否匹配选择器
		sel, _ := metav1.LabelSelectorAsSelector(item.Spec.TargetNamespaceSelector)
		if sel.Matches(labels.Set(ns.Labels)) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&item),
			})
		}
	}
	return requests
}

// SetupWithManager 设置控制器与管理器的关联
// 定义控制器监视哪些资源，以及如何处理这些资源的变化
func (r *SecretsyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// 主要关注 Secretsync 资源的变化
		For(&syncv1.Secretsync{}).
		// 监视 Secret 资源的变化，并通过 enqueueSecrets 确定需要调和的 Secretsync
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueSecrets),
		).
		// 监视 Namespace 资源的变化，并通过 enqueueNamespaces 确定需要调和的 Secretsync
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueNamespaces),
		).
		// 完成控制器设置
		Complete(r)
}
