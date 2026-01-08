package log

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// hook 로그 시스템의 Hook 인터페이스를 구현하여 로그 레벨에 따른 '계층적 분산 로깅'을 수행합니다.
//
// 단일 로그 이벤트를 중요도에 따라 Critical, Main, Verbose 채널로 선별적으로 라우팅하여,
// 운영 로그의 명확성을 보장하고 디버깅 정보와의 물리적 격리를 제공합니다.
//
// 핵심 역할:
//   - 로그 분기: Error 이상은 Critical 및 Main, Info 이상은 Main, Debug 이하는 Verbose 파일로 각각 기록합니다.
//   - 노이즈 차단: 상세 디버그 로그가 운영(Main) 로그에 유입되는 것을 방지하여 장애 분석 효율을 높입니다.
type hook struct {
	mainWriter     io.Writer // 운영 상태와 에러를 기록하는 메인 로깅 채널 (INFO / WARN / ERROR / FATAL / PANIC)
	criticalWriter io.Writer // 치명적 장애를 별도로 격리하여 보존하는 채널 (ERROR / FATAL / PANIC)
	verboseWriter  io.Writer // 상세 분석을 위한 디버깅 정보를 기록하는 채널 (DEBUG / TRACE)
	consoleWriter  io.Writer // 모든 레벨의 로그를 실시간으로 출력하는 표준 출력(Stdout)

	formatter Formatter

	mu sync.RWMutex // 로그 기록(Read Lock)과 종료 처리(Write Lock) 간의 동시성 제어

	closed bool // Hook의 종료 여부를 나타내며, true일 경우 모든 로그 기록 요청을 거부
}

// Levels 이 Hook이 수신할 로그 레벨의 집합을 반환합니다.
func (h *hook) Levels() []Level {
	return AllLevels
}

// Fire 발생한 로그 이벤트를 수신하여, 사전에 정의된 라우팅 정책(Level-based Routing)에 따라 적절한 Writer로 분배 및 기록합니다.
func (h *hook) Fire(entry *Entry) error {
	// Read Lock을 획득하여 동시 로깅을 허용하며, 작업 수행 중 Hook이 종료되지 않도록 보호합니다.
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil
	}

	// 로그 포맷팅 (한 번만 수행하여 재사용)
	msg, err := h.formatter.Format(entry)
	if err != nil {
		return err
	}

	var firstErr error

	// 0. Console Writer
	//    설정된 경우, 레벨 필터링 없이 모든 로그를 표준 출력으로 내보내 실시간 모니터링을 지원합니다.
	if h.consoleWriter != nil {
		// 표준 출력(Stdout) 쓰기가 실패해도 전체 로깅 시스템의 가용성에 영향을 주지 않도록, 발생한 에러를 전파하지 않고 의도적으로 무시합니다.
		if _, err := h.consoleWriter.Write(msg); err != nil {
			fmt.Fprintf(os.Stderr, "[LOG-SYSTEM-WARN] 표준 출력(Console) 쓰기 실패 (모니터링 제한 가능성): %v\n", err)
		}
	}

	// 1. Critical Writer (Error 이상)
	//    장애 대응을 위해 심각한 오류를 별도 파일에 격리 저장합니다.
	//    이 단계에서 쓰기 에러가 발생하더라도, 메인 로그 기록은 반드시 수행되어야 하므로 에러를 즉시 반환하지 않고 유예합니다.
	if entry.Level <= ErrorLevel {
		if h.criticalWriter != nil {
			if _, err := h.criticalWriter.Write(msg); err != nil {
				firstErr = err

				// 심각한 오류(Critical) 기록 실패는 데이터 유실을 의미하므로, 즉시 표준 에러로 알립니다.
				fmt.Fprintf(os.Stderr, "[LOG-SYSTEM-FAILURE] Critical 로그 파일 쓰기 실패 (데이터 유실 위험): %v\n", err)
			}
		}
	}

	// 2. Verbose Writer (Debug/Trace)
	//    디버깅 목적의 대량 상세 로그를 별도 파일로 분리합니다.
	//    중요: 처리 후 함수를 즉시 종료하여, 상세 정보가 메인 운영 로그를 오염시키지 않도록 원천 차단합니다.
	if entry.Level >= DebugLevel {
		if h.verboseWriter != nil {
			if _, err := h.verboseWriter.Write(msg); err != nil {
				if firstErr == nil {
					firstErr = err
				}

				// 상세 로그(Verbose) 기록 실패는 운영에 치명적이지 않으므로 경고 수준으로 알립니다.
				fmt.Fprintf(os.Stderr, "[LOG-SYSTEM-WARN] Verbose 로그 파일 쓰기 실패: %v\n", err)
			}
		}

		// 상세 로그(Debug/Trace)는 메인 로그에 남기지 않습니다.
		// 따라서 Main Writer로 넘어가지 않고 여기서 함수를 종료합니다.
		return firstErr
	}

	// 3. Main Writer (Info 이상)
	//    전반적인 시스템 운영 이력을 기록합니다. 앞선 단계의 에러 로그도 중복 기록하여 문맥을 보존합니다.
	//    Critical Writer의 실패 여부와 관계없이 기록 시도를 보장합니다.
	if h.mainWriter != nil {
		if _, err := h.mainWriter.Write(msg); err != nil {
			if firstErr == nil {
				firstErr = err
			}

			// 메인 로그(Main) 기록 실패는 운영 기록의 공백을 의미하므로 즉시 알립니다.
			fmt.Fprintf(os.Stderr, "[LOG-SYSTEM-FAILURE] Main 로그 파일 쓰기 실패 (운영 기록 유실 위험): %v\n", err)
		}
	}

	return firstErr
}

// Close Hook의 상태를 '종료(Closed)'로 전환하여 더 이상의 로그 기록을 차단하고, 내부 리소스에 대한 접근을 안전하게 정리합니다.
func (h *hook) Close() error {
	// Write Lock을 획득하여, 현재 실행 중인 모든 로깅 작업(Read Lock)이 완료될 때까지 대기합니다.
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true

	return nil
}
