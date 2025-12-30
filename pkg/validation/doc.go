/*
Package validation 애플리케이션 전반에서 사용되는 입력 데이터의 유효성을 검사하는 기능을 제공합니다.

이 패키지는 외부 입력값(설정 파일, API 요청 등)에 대한 신뢰성을 보장하기 위해 설계되었으며,
가능한 한 표준(Standard)과 보안 권장 사항을 엄격하게 준수하는 것을 목표로 합니다.

주요 기능:

  - CORS (Cross-Origin Resource Sharing) Origin 검증
  - Cron 표현식 (Extended/Quartz Format) 검증

사용 시 주의사항:

  - 모든 검증 함수는 유효하지 않은 입력에 대해 명확한 error를 반환합니다.
  - 패키지 내 함수들은 스레드 안전(Thread-Safe)하도록 설계되었습니다.
*/
package validation
