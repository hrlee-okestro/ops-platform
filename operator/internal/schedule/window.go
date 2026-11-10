// Package schedule은 ScheduledScaler의 시간 구간 판정 로직을 담는다.
// 클러스터와 무관한 순수 계산이라 go test만으로 검증할 수 있다.
package schedule

import "time"

// InWindow는 now가 [start, end) 구간 안에 있는지 판정한다.
//
//   - start, end는 24시간제 "HH:MM" 문자열이고, timezone은 IANA 이름(예: "Asia/Seoul")이다.
//   - start 시각은 구간에 포함하고, end 시각은 포함하지 않는다. (09:00~18:00이면 09:00은 안, 18:00은 밖)
//   - start가 end보다 늦으면(18:00~09:00) 자정을 넘어가는 구간이다.
//   - start와 end가 같으면 빈 구간이라 항상 false이다.
//   - 시각이나 시간대를 해석할 수 없으면 error를 돌려준다.
func InWindow(now time.Time, start, end, timezone string) (bool, error) {
	// 1단계: 시간대 이름을 시간대 객체로 바꾼다. 없는 이름이면 에러를 돌려준다.
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return false, err
	}

	// 2단계: "HH:MM"을 "자정부터 몇 분째"인 숫자로 바꾼다. 이러면 시각 비교가 정수 비교가 된다.
	startMin, err := minutesOfDay(start)
	if err != nil {
		return false, err
	}
	endMin, err := minutesOfDay(end)
	if err != nil {
		return false, err
	}

	// 3단계: 지금 시각을 그 시간대로 바꿔서 같은 방식으로 "몇 분째"를 구한다.
	local := now.In(loc)
	cur := local.Hour()*60 + local.Minute()

	// 4단계: 구간의 모양에 따라 판정한다.
	switch {
	case startMin == endMin: // 빈 구간
		return false, nil
	case startMin < endMin: // 같은 날 안의 구간 (예: 09:00~18:00)
		return startMin <= cur && cur < endMin, nil
	default: // 자정을 넘는 구간 (예: 18:00~09:00). 시작 이후이거나 종료 이전이면 안이다.
		return cur >= startMin || cur < endMin, nil
	}
}

// minutesOfDay는 "HH:MM"을 자정부터 지난 분(0~1439)으로 바꾼다. 형식이 틀리면 에러를 돌려준다.
// 소문자로 시작하므로 이 패키지 안에서만 쓰인다.
func minutesOfDay(hhmm string) (int, error) {
	// Go는 형식을 "15:04"라는 기준 예시로 지정한다 (15시 04분). HH:mm 같은 기호가 아니다.
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}
