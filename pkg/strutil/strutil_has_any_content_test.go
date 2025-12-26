package strutil

import "testing"

func TestHasAnyContent(t *testing.T) {
	t.Parallel() // 병렬 실행을 통해 테스트 속도 향상

	type args struct {
		strs []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// [Category 1] 기본 동작 검증 (Basic Behavior)
		{
			name: "단일 문자열 입력 시 내용이 있으면 true 반환",
			args: args{strs: []string{"hello"}},
			want: true,
		},
		{
			name: "단일 문자열 입력 시 내용이 없으면 false 반환",
			args: args{strs: []string{""}},
			want: false,
		},
		{
			name: "여러 문자열 중 하나라도 내용이 있으면 true 반환 (중간에 포함)",
			args: args{strs: []string{"", "world", ""}},
			want: true,
		},

		// [Category 2] 엣지 케이스 검증 (Edge Cases)
		{
			name: "인자가 아예 전달되지 않은 경우 false 반환 (Nil Slice)",
			args: args{strs: nil},
			want: false,
		},
		{
			name: "인자가 아예 전달되지 않은 경우 false 반환 (Empty Slice)",
			args: args{strs: []string{}},
			want: false,
		},
		{
			name: "모든 인자가 빈 문자열인 경우 false 반환",
			args: args{strs: []string{"", "", "", ""}},
			want: false,
		},
		{
			name: "공백 문자(Whitespace)만 있어도 내용이 있는 것으로 간주 (Trim하지 않음)",
			args: args{strs: []string{"   "}},
			want: true, // 주의: HasAnyContent는 Trim을 수행하지 않음
		},
	}

	for _, tt := range tests {
		tt := tt // 캡처링 문제 방지 (Go 1.22 미만 호환성 고려)
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HasAnyContent(tt.args.strs...); got != tt.want {
				t.Errorf("HasAnyContent() = %v, want %v", got, tt.want)
			}
		})
	}
}
