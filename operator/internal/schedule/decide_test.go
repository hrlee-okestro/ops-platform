package schedule

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opsv1alpha1 "github.com/hrlee-okestro/ops-platform/operator/api/v1alpha1"
)

func int32Ptr(v int32) *int32 { return &v }

func TestDecide(t *testing.T) {
	// 시간표는 UTC 18:00~09:00에 replicas 0
	newSS := func(original *int32, override *opsv1alpha1.Override) *opsv1alpha1.ScheduledScaler {
		return &opsv1alpha1.ScheduledScaler{
			Spec: opsv1alpha1.ScheduledScalerSpec{
				Timezone:  "UTC",
				ScaleDown: opsv1alpha1.ScaleDownWindow{Start: "18:00", End: "09:00", Replicas: 0},
				Override:  override,
			},
			Status: opsv1alpha1.ScheduledScalerStatus{OriginalReplicas: original},
		}
	}
	at := func(h, m int) time.Time { return time.Date(2026, 1, 5, h, m, 0, 0, time.UTC) }
	until := func(h, m int) *opsv1alpha1.Override {
		return &opsv1alpha1.Override{Replicas: 3, Until: metav1.NewTime(at(h, m))}
	}

	tests := []struct {
		name         string
		ss           *opsv1alpha1.ScheduledScaler
		now          time.Time
		current      int32
		wantDesired  int32
		wantPhase    opsv1alpha1.Phase
		wantOriginal *int32
		wantRequeue  time.Duration
		wantErr      bool
	}{
		{"평소: 아무것도 안 함", newSS(nil, nil), at(12, 0), 2, 2, opsv1alpha1.PhaseNormal, nil, time.Minute, false},
		{"구간 진입: 원래 값을 기억하고 줄임", newSS(nil, nil), at(20, 0), 2, 0, opsv1alpha1.PhaseScaledDown, int32Ptr(2), time.Minute, false},
		{"구간 중: 기억한 값 유지", newSS(int32Ptr(2), nil), at(21, 0), 0, 0, opsv1alpha1.PhaseScaledDown, int32Ptr(2), time.Minute, false},
		{"구간 종료: 원래 값으로 복원하고 기억 삭제", newSS(int32Ptr(2), nil), at(9, 0), 0, 2, opsv1alpha1.PhaseNormal, nil, time.Minute, false},
		{"override 시작(평소 시간대): 원래 값 기억", newSS(nil, until(13, 0)), at(12, 0), 2, 3, opsv1alpha1.PhaseOverridden, int32Ptr(2), time.Minute, false},
		{"override가 곧 끝남: 끝나는 시각에 깨움", newSS(int32Ptr(2), until(12, 0)), at(11, 59), 3, 3, opsv1alpha1.PhaseOverridden, int32Ptr(2), time.Minute, false},
		{"override가 30초 뒤 끝남", newSS(int32Ptr(2), until(12, 0)), at(12, 0).Add(-30 * time.Second), 3, 3, opsv1alpha1.PhaseOverridden, int32Ptr(2), 30 * time.Second, false},
		{"override 만료(평소 시간대): 원래 값으로 복원", newSS(int32Ptr(2), until(12, 0)), at(12, 1), 3, 2, opsv1alpha1.PhaseNormal, nil, time.Minute, false},
		{"override 만료(구간 중): 시간표대로 줄임", newSS(int32Ptr(2), until(19, 0)), at(19, 1), 3, 0, opsv1alpha1.PhaseScaledDown, int32Ptr(2), time.Minute, false},
		{"override는 시간표보다 우선", newSS(int32Ptr(2), until(23, 0)), at(20, 0), 0, 3, opsv1alpha1.PhaseOverridden, int32Ptr(2), time.Minute, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decide(tc.ss, tc.now, tc.current)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if got.Desired != tc.wantDesired {
				t.Errorf("Desired: got %d, want %d", got.Desired, tc.wantDesired)
			}
			if got.Phase != tc.wantPhase {
				t.Errorf("Phase: got %s, want %s", got.Phase, tc.wantPhase)
			}
			if (got.OriginalReplicas == nil) != (tc.wantOriginal == nil) ||
				(got.OriginalReplicas != nil && *got.OriginalReplicas != *tc.wantOriginal) {
				t.Errorf("OriginalReplicas: got %v, want %v", got.OriginalReplicas, tc.wantOriginal)
			}
			if got.RequeueAfter != tc.wantRequeue {
				t.Errorf("RequeueAfter: got %s, want %s", got.RequeueAfter, tc.wantRequeue)
			}
		})
	}

	t.Run("잘못된 시간대는 에러", func(t *testing.T) {
		ss := newSS(nil, nil)
		ss.Spec.Timezone = "Mars/Base"
		if _, err := Decide(ss, at(12, 0), 2); err == nil {
			t.Fatal("에러가 나야 한다")
		}
	})
}
