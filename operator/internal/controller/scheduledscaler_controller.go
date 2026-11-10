/*
Copyright 2026.

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

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	opsv1alpha1 "github.com/hrlee-okestro/ops-platform/operator/api/v1alpha1"
	"github.com/hrlee-okestro/ops-platform/operator/internal/schedule"
)

// conditionAvailable은 "대상 Deployment를 정상적으로 조정하고 있는가"를 나타내는 condition 종류이다.
const conditionAvailable = "Available"

// retryInterval은 대상이 없거나 일시적으로 실패했을 때 다시 시도하기까지의 대기 시간이다.
const retryInterval = time.Minute

// ScheduledScalerReconciler는 ScheduledScaler CR을 보고 대상 Deployment의 replicas를 맞춘다.
type ScheduledScalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ops.okestro.dev,resources=scheduledscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ops.okestro.dev,resources=scheduledscalers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ops.okestro.dev,resources=scheduledscalers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch

// Reconcile은 ScheduledScaler가 바뀌거나 예약한 시각이 되면 호출된다.
// "이벤트가 무엇이었나"가 아니라 "지금 상태가 무엇인가"를 읽어서 원하는 상태로 맞춘다 (level-based).
// 같은 입력으로 여러 번 호출돼도 결과가 같아야 한다 (멱등).
func (r *ScheduledScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. 지금 CR을 읽는다. 이미 삭제됐다면 할 일이 없다.
	var ss opsv1alpha1.ScheduledScaler
	if err := r.Get(ctx, req.NamespacedName, &ss); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. 대상 Deployment를 읽는다. CR과 같은 네임스페이스에서만 찾는다 (다른 네임스페이스는 건드릴 수 없다).
	var dep appsv1.Deployment
	depKey := types.NamespacedName{Namespace: req.Namespace, Name: ss.Spec.TargetRef.Name}
	if err := r.Get(ctx, depKey, &dep); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Target Deployment not found", "deployment", depKey.Name)
			return ctrl.Result{RequeueAfter: retryInterval}, r.setCondition(ctx, &ss, metav1.ConditionFalse,
				"TargetNotFound", fmt.Sprintf("Deployment %q not found", depKey.Name))
		}
		return ctrl.Result{}, err
	}
	current := int32(1) // replicas를 생략한 Deployment의 기본값
	if dep.Spec.Replicas != nil {
		current = *dep.Spec.Replicas
	}

	// 3. 지금 몇 개여야 하는지 계산한다 (순수 함수, internal/schedule/decide.go).
	d, err := schedule.Decide(&ss, time.Now(), current)
	if err != nil {
		// spec이 잘못된 경우(예: 없는 시간대)는 다시 시도해도 소용없다. 사용자가 고치면 다시 호출된다.
		log.Error(err, "Invalid schedule", "scheduledscaler", req.NamespacedName)
		return ctrl.Result{}, r.setCondition(ctx, &ss, metav1.ConditionFalse, "InvalidSchedule", err.Error())
	}

	// 4. 다르면 Deployment의 replicas를 바꾼다.
	scaled := false
	if d.Desired != current {
		base := dep.DeepCopy()
		dep.Spec.Replicas = &d.Desired
		if err := r.Patch(ctx, &dep, client.MergeFrom(base)); err != nil {
			return ctrl.Result{}, err
		}
		scaled = true
		log.Info("Scaled Deployment", "deployment", depKey.Name, "from", current, "to", d.Desired, "phase", d.Phase)
	}

	// 5. 관측한 결과를 status에 기록한다.
	ss.Status.Phase = d.Phase
	ss.Status.OriginalReplicas = d.OriginalReplicas
	if scaled {
		now := metav1.Now()
		ss.Status.LastScaleTime = &now
	}
	if err := r.setCondition(ctx, &ss, metav1.ConditionTrue, "Reconciled", "Deployment replicas match the desired state"); err != nil {
		return ctrl.Result{}, err
	}

	// 6. 다음에 다시 확인할 시각을 예약한다.
	return ctrl.Result{RequeueAfter: d.RequeueAfter}, nil
}

// setCondition은 Available condition을 갱신하고 status를 저장한다.
func (r *ScheduledScalerReconciler) setCondition(ctx context.Context, ss *opsv1alpha1.ScheduledScaler, status metav1.ConditionStatus, reason, message string) error {
	meta.SetStatusCondition(&ss.Status.Conditions, metav1.Condition{
		Type:               conditionAvailable,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: ss.Generation,
	})
	return r.Status().Update(ctx, ss)
}

// SetupWithManager는 이 컨트롤러를 매니저에 등록한다.
// ScheduledScaler가 만들어지거나 바뀌면 Reconcile이 호출된다.
func (r *ScheduledScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1alpha1.ScheduledScaler{}).
		Named("scheduledscaler").
		Complete(r)
}
