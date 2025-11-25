package controller

import (
	"context"
	"time"

	cleanerv1alpha1 "github.com/Uk-jake/Operator_Project/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// CleanPolicyReconciler reconciles a CleanPolicy object
type CleanPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *CleanPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cleanerv1alpha1.CleanPolicy{}).
		Named("cleanpolicy").
		Complete(r)
}

// +kubebuilder:rbac:groups=cleaner.cloudclub.dev,resources=cleanpolicies,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cleaner.cloudclub.dev,resources=cleanpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cleaner.cloudclub.dev,resources=cleanpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;delete

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *CleanPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Reconciling CleanPolicy", "name", req.NamespacedName)

	// 1. CleanPolicy 객체 가져오기
	var policy cleanerv1alpha1.CleanPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			// 리소스가 삭제된 경우: 추가 작업 필요 없음
			logger.Info("CleanPolicy resource not found, might have been deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch CleanPolicy")
		return ctrl.Result{}, err
	}

	// 2. schedule 파싱 ("1h", "30m" 등)
	duration, err := time.ParseDuration(policy.Spec.Schedule)
	if err != nil {
		logger.Error(err, "invalid schedule format", "schedule", policy.Spec.Schedule)
		// 포맷이 잘못되면, 너무 자주 돌지 않도록 1분 뒤 재시도
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// 3. targetNamespaces 설정
	targetNamespaces := []string{}
	if len(policy.Spec.TargetNamespaces) > 0 {
		targetNamespaces = policy.Spec.TargetNamespaces
	} else {
		// 명시되지 않으면 CleanPolicy가 존재하는 네임스페이스만 타겟
		targetNamespaces = []string{policy.Namespace}
	}
	logger.Info("Target namespaces resolved", "namespaces", targetNamespaces)

	// 4. JobSelector 라벨 셀렉터 준비
	var selector labels.Selector
	if policy.Spec.JobSelector != nil {
		selector, err = metav1.LabelSelectorAsSelector(policy.Spec.JobSelector)
		if err != nil {
			logger.Error(err, "invalid jobSelector in CleanPolicy")
			// 셀렉터가 잘못되면 재시도해도 소용 없으니 에러 리턴
			return ctrl.Result{}, err
		}
	} else {
		selector = labels.Everything()
	}
	logger.Info("Job selector prepared", "selector", selector.String())

	// 5. TTL 계산용 값 준비
	ttl := time.Duration(policy.Spec.TTLHours) * time.Hour
	now := time.Now()

	// 6. 네임스페이스별로 Job 조회 + 필터링
	var candidates []client.ObjectKey

	for _, ns := range targetNamespaces {
		var jobList batchv1.JobList

		// 해당 네임스페이스의 Job 조회 (label selector 적용)
		if err := r.List(
			ctx,
			&jobList,
			client.InNamespace(ns),
			client.MatchingLabelsSelector{Selector: selector},
		); err != nil {
			logger.Error(err, "failed to list Jobs", "namespace", ns)
			return ctrl.Result{}, err
		}

		for _, job := range jobList.Items {
			// 6-1. Active Job은 건드리지 않음
			if job.Status.Active > 0 {
				continue
			}

			// 6-2. CompletionTime 없는 Job은 스킵
			if job.Status.CompletionTime == nil {
				continue
			}

			// 6-3. Completed / Failed 상태만 대상
			if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
				continue
			}

			age := now.Sub(job.Status.CompletionTime.Time)
			if age < ttl {
				// 아직 TTL이 지나지 않음
				continue
			}

			// 여기까지 왔다면: 삭제 후보
			key := client.ObjectKey{Namespace: job.Namespace, Name: job.Name}
			candidates = append(candidates, key)
		}
	}

	logger.Info("Job cleanup candidates found",
		"count", len(candidates),
		"candidates", candidates)

	// 7. 실제 Job 삭제 수행
	var deleted int32 = 0

	for _, key := range candidates {
		// 한 번 더 Get 해서 최신 상태 확인 후 삭제
		var job batchv1.Job
		if err := r.Get(ctx, key, &job); err != nil {
			if errors.IsNotFound(err) {
				// 이미 삭제된 Job이면 무시
				logger.Info("Job already deleted", "jobNamespace", key.Namespace, "jobName", key.Name)
				continue
			}
			logger.Error(err, "failed to get Job for deletion", "jobNamespace", key.Namespace, "jobName", key.Name)
			continue
		}

		if err := r.Delete(ctx, &job); err != nil {
			logger.Error(err, "failed to delete Job", "jobNamespace", key.Namespace, "jobName", key.Name)
			continue
		}

		deleted++
	}

	logger.Info("Job cleanup completed", "deletedCount", deleted)

	// 8. Status 업데이트 (마지막 실행 시간, 삭제 개수, 누적 삭제 개수)
	nowTime := metav1.NewTime(now)
	policy.Status.LastCleanupTime = &nowTime
	policy.Status.LastRunDeleted = deleted
	policy.Status.TotalCleanedJobs += int64(deleted)

	if err := r.Status().Update(ctx, &policy); err != nil {
		logger.Error(err, "failed to update CleanPolicy status")
		// 상태 업데이트 실패는 치명적이진 않지만, 일단 에러로 보고 재시도하게 두자
		return ctrl.Result{}, err
	}

	logger.Info("Reconcile logic completed with cleanup and status update",
		"lastRunDeleted", deleted,
		"totalCleanedJobs", policy.Status.TotalCleanedJobs)

	// 9. 다음 Reconcile 실행 시점 예약
	return ctrl.Result{RequeueAfter: duration}, nil
}
