package lotto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPredictionSnapshot_Instantiation(t *testing.T) {
	// 구조체 인스턴스화 테스트
	snapshot := &predictionSnapshot{}
	assert.NotNil(t, snapshot)
}

func TestPredictionSnapshot_JSONCheck(t *testing.T) {
	// JSON 마샬링/언마샬링 호환성 테스트
	// 현재는 빈 구조체이므로 "{}"가 되어야 함을 보장하여,
	// 향후 필드 추가 시 하위 호환성이나 예상치 못한 변경을 감지할 수 있게 합니다.

	snapshot := &predictionSnapshot{}

	// Marshal
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	assert.JSONEq(t, "{}", string(data))

	// Unmarshal
	var unmarshalled predictionSnapshot
	err = json.Unmarshal(data, &unmarshalled)
	require.NoError(t, err)
	assert.Equal(t, *snapshot, unmarshalled)
}

func TestPredictionSnapshot_ImplementsAny(t *testing.T) {
	// Task 설정에서 NewSnapshot이 반환하는 타입이 any(interface{})이므로,
	// 해당 타입으로 사용 가능한지 확인합니다.
	var _ any = (*predictionSnapshot)(nil)
}
