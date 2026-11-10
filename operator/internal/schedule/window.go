// Package schedule은 ScheduledScaler의 시간 구간 판정 로직을 담는다.
// 클러스터와 무관한 순수 계산이라 go test만으로 검증할 수 있다.
package schedule

import (
	"errors"
	"time"
)

// InWindow는 now가 [start, end) 구간 안에 있는지 판정한다.
//
//   - start, end는 24시간제 "HH:MM" 문자열이고, timezone은 IANA 이름(예: "Asia/Seoul")이다.
//   - start 시각은 구간에 포함하고, end 시각은 포함하지 않는다. (09:00~18:00이면 09:00은 안, 18:00은 밖)
//   - start가 end보다 늦으면(18:00~09:00) 자정을 넘어가는 구간이다.
//   - start와 end가 같으면 빈 구간이라 항상 false이다.
//   - 시각이나 시간대를 해석할 수 없으면 error를 돌려준다.
func InWindow(now time.Time, start, end, timezone string) (bool, error) {
	// TODO(사용자): 직접 구현한다. 힌트는 아래 설명을 참고.
	return false, errors.New("아직 구현하지 않음")
}
