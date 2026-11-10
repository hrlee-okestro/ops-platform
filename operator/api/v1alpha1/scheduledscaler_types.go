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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// 이 파일은 ScheduledScaler CRD(사용자가 쓰는 YAML의 형식)를 정의한다.
// - 필드 위의 `json:"..."` 태그가 곧 YAML의 키 이름이다.
// - `+kubebuilder:...` 주석(마커)은 CRD YAML을 만들 때 검증 규칙으로 변환된다.
// - 이 파일을 고친 뒤에는 `make manifests generate`로 CRD YAML과 DeepCopy 코드를 다시 만든다.

// TargetRef는 크기를 조정할 Deployment를 가리킨다.
// 네임스페이스 필드를 일부러 두지 않았다. CR과 같은 네임스페이스의 Deployment만 대상이 되므로
// 다른 네임스페이스의 워크로드를 가리킬 방법 자체가 없다 (CLAUDE.md 규칙 6).
type TargetRef struct {
	// name은 대상 Deployment의 이름이다.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// ScaleDownWindow는 replicas를 줄여 두는 시간 구간이다.
// start가 end보다 늦으면(예: 18:00~09:00) 자정을 넘어가는 구간으로 해석한다.
type ScaleDownWindow struct {
	// start는 구간 시작 시각이다 (24시간제 "HH:MM").
	// +kubebuilder:validation:Pattern=`^([01][0-9]|2[0-3]):[0-5][0-9]$`
	Start string `json:"start"`

	// end는 구간 종료 시각이다 (24시간제 "HH:MM"). 이 시각부터는 원래 replicas로 되돌린다.
	// +kubebuilder:validation:Pattern=`^([01][0-9]|2[0-3]):[0-5][0-9]$`
	End string `json:"end"`

	// replicas는 구간 안에서 유지할 replicas 수이다. 0이면 완전히 내린다.
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
}

// Override는 시간표보다 우선하는 임시 지시이다 (장애 조치용).
// until이 지나면 Operator가 알아서 무시하고 시간표로 돌아가므로, 승인된 조치가 영원히 남지 않는다.
type Override struct {
	// replicas는 until까지 고정할 replicas 수이다.
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`

	// until은 이 지시가 유효한 마지막 시각이다 (RFC3339, 예: 2026-11-10T15:00:00Z).
	Until metav1.Time `json:"until"`
}

// ScheduledScalerSpec은 사용자가 원하는 상태(선언)이다. 사용자나 console-api가 쓰고, Operator는 읽기만 한다.
type ScheduledScalerSpec struct {
	// targetRef는 조정할 Deployment이다.
	// +required
	TargetRef TargetRef `json:"targetRef"`

	// timezone은 start/end를 해석할 IANA 시간대 이름이다 (예: Asia/Seoul). 생략하면 UTC이다.
	// +kubebuilder:default=UTC
	// +optional
	Timezone string `json:"timezone,omitempty"`

	// scaleDown은 replicas를 줄여 두는 구간이다. 구간 밖에서는 원래 replicas(status.originalReplicas)로 되돌린다.
	// +required
	ScaleDown ScaleDownWindow `json:"scaleDown"`

	// override가 있고 아직 until 전이면 시간표보다 우선해서 그 replicas로 맞춘다.
	// 장애 조치(예: 승인된 "replicas를 3으로 올려라")를 CR 필드 하나로 표현하기 위한 것이다.
	// +optional
	Override *Override `json:"override,omitempty"`
}

// Phase는 Operator가 판단한 현재 구간이다.
// +kubebuilder:validation:Enum=Normal;ScaledDown;Overridden
type Phase string

const (
	// PhaseNormal은 구간 밖이라 원래 replicas를 유지하는 상태이다.
	PhaseNormal Phase = "Normal"
	// PhaseScaledDown은 구간 안이라 replicas를 줄여 둔 상태이다.
	PhaseScaledDown Phase = "ScaledDown"
	// PhaseOverridden은 spec.override가 유효해서 시간표를 무시하고 있는 상태이다.
	PhaseOverridden Phase = "Overridden"
)

// ScheduledScalerStatus는 Operator가 관측해서 기록하는 현재 상태이다. 사용자는 쓰지 않는다.
type ScheduledScalerStatus struct {
	// phase는 지금 어느 구간인지를 나타낸다.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// originalReplicas는 줄이기 직전의 replicas이다. 구간이 끝나면 이 값으로 되돌린다.
	// 0도 의미 있는 값이라 포인터로 둔다. nil이면 "아직 기록한 적 없음"이다.
	// +optional
	OriginalReplicas *int32 `json:"originalReplicas,omitempty"`

	// lastScaleTime은 Operator가 마지막으로 replicas를 바꾼 시각이다.
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// conditions는 리소스의 세부 상태 목록이다. 각 condition은 type별로 하나씩이며
	// 상태 값은 True, False, Unknown 중 하나이다.
	// 흔히 쓰는 type: Available(정상 동작), Progressing(변경 중), Degraded(원하는 상태에 못 도달).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ScheduledScaler는 시간대별로 Deployment의 replicas를 조정하는 정책 리소스이다.
type ScheduledScaler struct {
	metav1.TypeMeta `json:",inline"`

	// metadata는 이름, 네임스페이스, 라벨 같은 Kubernetes 공통 메타데이터이다.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec은 원하는 상태이다 (사용자가 씀).
	// +required
	Spec ScheduledScalerSpec `json:"spec"`

	// status는 관측한 현재 상태이다 (Operator가 씀).
	// +optional
	Status ScheduledScalerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ScheduledScalerList는 ScheduledScaler 목록이다 (kubectl get, List API 응답용).
type ScheduledScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ScheduledScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ScheduledScaler{}, &ScheduledScalerList{})
		return nil
	})
}
