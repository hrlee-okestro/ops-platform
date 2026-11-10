package schedule

import (
	"time"

	opsv1alpha1 "github.com/hrlee-okestro/ops-platform/operator/api/v1alpha1"
)

// pollInterval은 다음 재조정까지의 기본 대기 시간이다.
// 1바퀴 범위의 단순화: 구간 경계 시각을 정확히 계산해 예약하는 대신 1분마다 상태를 다시 본다.
// (2바퀴에서 "다음 경계 시각"을 계산해 그 시각에 깨우도록 개선한다.)
const pollInterval = time.Minute

// Decision은 Decide의 결과이다. Reconciler는 이 값대로 Deployment와 status를 바꾼다.
type Decision struct {
	// Desired는 Deployment가 가져야 할 replicas이다.
	Desired int32
	// Phase는 status.phase에 기록할 값이다.
	Phase opsv1alpha1.Phase
	// OriginalReplicas는 status.originalReplicas에 기록할 값이다 (nil이면 비운다).
	OriginalReplicas *int32
	// RequeueAfter는 다음 재조정까지 기다릴 시간이다.
	RequeueAfter time.Duration
}

// Decide는 "지금 이 Deployment는 몇 개여야 하는가"를 계산하는 순수 함수이다.
// 클러스터에 접근하지 않고 입력만으로 결과가 정해지므로 go test만으로 검증할 수 있다.
//
// 우선순위: override(유효 기간 중) > 시간표(scaleDown 구간) > 평소(원래 replicas).
// 줄이거나 덮어쓰는 동안에는 원래 replicas를 status에 기억해 두었다가, 평소로 돌아오면 그 값으로 되돌린다.
// current는 지금 Deployment의 replicas이다.
func Decide(ss *opsv1alpha1.ScheduledScaler, now time.Time, current int32) (Decision, error) {
	spec := ss.Spec
	requeue := pollInterval

	var desired int32
	phase := opsv1alpha1.PhaseNormal

	if spec.Override != nil && now.Before(spec.Override.Until.Time) {
		desired, phase = spec.Override.Replicas, opsv1alpha1.PhaseOverridden
		// override가 끝나는 시각에 맞춰 깨어나도록 대기 시간을 줄인다.
		if untilEnd := spec.Override.Until.Sub(now); untilEnd < requeue {
			requeue = untilEnd
		}
	} else {
		inWindow, err := InWindow(now, spec.ScaleDown.Start, spec.ScaleDown.End, spec.Timezone)
		if err != nil {
			return Decision{}, err
		}
		if inWindow {
			desired, phase = spec.ScaleDown.Replicas, opsv1alpha1.PhaseScaledDown
		}
	}

	original := ss.Status.OriginalReplicas

	// 평소 상태: 기억해 둔 원래 값이 있으면 되돌리고, 없으면 지금 값을 그대로 둔다.
	if phase == opsv1alpha1.PhaseNormal {
		desired = current
		if original != nil {
			desired = *original
		}
		return Decision{Desired: desired, Phase: phase, OriginalReplicas: nil, RequeueAfter: requeue}, nil
	}

	// 줄이는/덮어쓰는 상태: 처음 진입할 때만 원래 값을 기억한다 (이미 있으면 유지).
	if original == nil {
		saved := current
		original = &saved
	} else {
		kept := *original
		original = &kept
	}
	return Decision{Desired: desired, Phase: phase, OriginalReplicas: original, RequeueAfter: requeue}, nil
}
