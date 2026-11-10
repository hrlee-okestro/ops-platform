package schedule

import (
	"testing"
	"time"
)

func TestInWindow(t *testing.T) {
	// UTC 기준 2026-01-05의 h시 m분
	utc := func(h, m int) time.Time { return time.Date(2026, 1, 5, h, m, 0, 0, time.UTC) }

	tests := []struct {
		name    string
		now     time.Time
		start   string
		end     string
		tz      string
		want    bool
		wantErr bool
	}{
		// 같은 날 안의 구간: 09:00 ~ 18:00
		{"낮 구간, 시작 직전", utc(8, 59), "09:00", "18:00", "UTC", false, false},
		{"낮 구간, 시작 정각", utc(9, 0), "09:00", "18:00", "UTC", true, false},
		{"낮 구간, 종료 직전", utc(17, 59), "09:00", "18:00", "UTC", true, false},
		{"낮 구간, 종료 정각", utc(18, 0), "09:00", "18:00", "UTC", false, false},

		// 자정을 넘는 구간: 18:00 ~ 09:00
		{"밤 구간, 시작 직전", utc(17, 59), "18:00", "09:00", "UTC", false, false},
		{"밤 구간, 시작 정각", utc(18, 0), "18:00", "09:00", "UTC", true, false},
		{"밤 구간, 자정 전", utc(23, 30), "18:00", "09:00", "UTC", true, false},
		{"밤 구간, 자정 후", utc(2, 0), "18:00", "09:00", "UTC", true, false},
		{"밤 구간, 종료 직전", utc(8, 59), "18:00", "09:00", "UTC", true, false},
		{"밤 구간, 종료 정각", utc(9, 0), "18:00", "09:00", "UTC", false, false},

		// 시간대: UTC 09:30은 서울 18:30이다 (UTC+9)
		{"서울 시간대로 해석", utc(9, 30), "18:00", "09:00", "Asia/Seoul", true, false},
		{"같은 시각을 UTC로 해석", utc(9, 30), "18:00", "09:00", "UTC", false, false},

		// 빈 구간
		{"시작과 종료가 같음", utc(12, 0), "12:00", "12:00", "UTC", false, false},

		// 잘못된 입력은 error
		{"없는 시각", utc(12, 0), "25:00", "09:00", "UTC", false, true},
		{"형식이 다름", utc(12, 0), "6pm", "09:00", "UTC", false, true},
		{"없는 시간대", utc(12, 0), "18:00", "09:00", "Mars/Base", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := InWindow(tc.now, tc.start, tc.end, tc.tz)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error 여부가 다름: err=%v, wantErr=%v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
