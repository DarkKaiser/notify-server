pipeline {

    agent any

    environment {
        PROJECT_NAME = "NotifyServer"
        BUILD_TIMESTAMP = sh(script: "date -u +'%Y-%m-%dT%H:%M:%SZ'", returnStdout: true).trim()
    }

    stages {

        stage('환경 검증') {
            steps {
                script {
                    // 필수 환경 변수 확인
                    echo "환경 변수 검증 중..."
                    
                    if (!env.TELEGRAM_BOT_TOKEN) {
                        error("필수 환경 변수가 설정되지 않았습니다: TELEGRAM_BOT_TOKEN")
                    }
                    
                    if (!env.TELEGRAM_CHAT_ID) {
                        error("필수 환경 변수가 설정되지 않았습니다: TELEGRAM_CHAT_ID")
                    }
                    
                    echo "환경 검증 완료"
                    echo "빌드 타임스탬프: ${env.BUILD_TIMESTAMP}"
                }
            }
        }

        stage('준비') {
            steps {
                cleanWs()
            }
        }

        stage('소스 체크아웃') {
            steps {
                checkout([
                    $class: 'GitSCM',
                    branches: [[ name: '*/master' ]],
                    extensions: [[
                        $class: 'SubmoduleOption',
                        disableSubmodules: false,
                        parentCredentials: true,
                        recursiveSubmodules: false,
                        reference: '',
                        trackingSubmodules: true
                    ]],
                    submoduleCfg: [],
                    userRemoteConfigs: [[
                        credentialsId: 'github-darkkaiser-credentials',
                        url: 'https://github.com/DarkKaiser/notify-server.git'
                    ]]
                ])
                
                // Git 정보 설정
                script {
                    env.GIT_COMMIT_SHORT = sh(script: "git rev-parse --short HEAD", returnStdout: true).trim()
                    env.GIT_COMMIT_FULL = sh(script: "git rev-parse HEAD", returnStdout: true).trim()
                    
                    echo "Git 정보:"
                    echo "  커밋: ${env.GIT_COMMIT_SHORT} (${env.GIT_COMMIT_FULL})"
                    echo "  빌드: #${env.BUILD_NUMBER}"
                }
            }
        }

        stage('테스트 및 코드 품질 검사') {
            steps {
                script {
                    // Dockerfile의 builder 스테이지에서 테스트와 린트(golangci-lint)가 모두 실행됩니다.
                    // 이를 통해 Jenkins 환경(DinD)에서의 볼륨 마운트 문제를 해결하고, 일관된 검사 환경을 보장합니다.
                    
                    echo "빌드 및 테스트 시작..."
                    sh """
                        docker build --target builder \\
                            --build-arg GIT_COMMIT=${env.GIT_COMMIT_SHORT} \\
                            --build-arg BUILD_DATE=${env.BUILD_TIMESTAMP} \\
                            --build-arg BUILD_NUMBER=${env.BUILD_NUMBER} \\
                            -t notify-server:test .
                    """
                    
                    echo "테스트 실행 중..."
                    sh 'docker run --rm notify-server:test go test ./... -v'
                }
            }
        }

        /*
        stage('보안 스캔') {
            steps {
                // Trivy를 사용하여 빌드된 이미지(notify-server:test)의 취약점을 스캔합니다.
                // 파일 시스템 스캔(fs) 대신 이미지 스캔(image)을 사용하여 볼륨 마운트 문제를 회피합니다.
                // --exit-code 0: 취약점이 발견되어도 빌드를 실패시키지 않고 경고만 남깁니다.
                // --severity HIGH,CRITICAL: 심각도가 높거나 치명적인 취약점만 검사합니다.
                sh 'docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:latest image --exit-code 0 --severity HIGH,CRITICAL notify-server:test'
            }
        }
        */
        
        // 보안 스캔 단계는 실행 시간이 오래 걸려 주석 처리했습니다.
        // 필요 시 수동으로 실행하거나, 야간 빌드 등 별도 스케줄로 분리하는 것을 권장합니다.
        
        stage('테스트 이미지 정리') {
            steps {
                // 테스트용 이미지는 더 이상 필요하지 않으므로 삭제
                // || true를 사용하여 이미지가 없어도 에러가 발생하지 않도록 함
                sh 'docker rmi notify-server:test || true'
            }
        }
        
        stage('도커 이미지 빌드') {
            steps {
                script {
                    echo "프로덕션 이미지 빌드 중..."
                    sh """
                        docker build \\
                            --build-arg GIT_COMMIT=${env.GIT_COMMIT_SHORT} \\
                            --build-arg BUILD_DATE=${env.BUILD_TIMESTAMP} \\
                            --build-arg BUILD_NUMBER=${env.BUILD_NUMBER} \\
                            -t darkkaiser/notify-server:latest \\
                            -t darkkaiser/notify-server:${env.GIT_COMMIT_SHORT} \\
                            .
                    """
                    echo "이미지 빌드 완료"
                }
            }
        }

        stage('도커 컨테이너 실행') {
            steps {
                script {
                    // 기존 컨테이너 중지 및 제거 (안전한 방식)
                    sh '''
                        if docker ps -a --filter name=notify-server --format '{{.Names}}' | grep -q '^notify-server$'; then
                            echo "기존 컨테이너 중지 중..."
                            docker container stop notify-server || true
                            echo "기존 컨테이너 제거 중..."
                            docker container rm notify-server || true
                        else
                            echo "기존 컨테이너가 없습니다."
                        fi
                    '''
                    
                    // 새 컨테이너 실행
                    echo "새 컨테이너 시작 중..."
                    sh '''
                        docker run -d --name notify-server \\
                                      -e TZ=Asia/Seoul \\
                                      -v /usr/local/docker/notify-server:/usr/local/app \\
                                      -v /usr/local/docker/nginx-proxy-manager/letsencrypt:/etc/letsencrypt:ro \\
                                      -p 2443:2443 \\
                                      --restart="always" \\
                                      darkkaiser/notify-server:latest
                    '''
                    
                    // 컨테이너 상태 확인
                    sh '''
                        echo "컨테이너 상태 확인 중..."
                        sleep 3
                        docker ps --filter name=notify-server --format 'table {{.Names}}\\t{{.Status}}\\t{{.Ports}}'
                    '''
                }
            }
        }

        stage('도커 이미지 정리') {
            steps {
                // dangling 이미지 정리
                sh 'docker images -qf dangling=true | xargs -r docker rmi || echo "정리할 dangling 이미지가 없습니다."'
            }
        }
        
    }

    post {

        success {
            script {
                def message = """【 알림 > Jenkins > ${env.PROJECT_NAME} 】

✅ 빌드 작업이 성공하였습니다.

커밋: ${env.GIT_COMMIT_SHORT}
빌드: #${env.BUILD_NUMBER}
시간: ${env.BUILD_TIMESTAMP}

${env.BUILD_URL}"""
                
                sh """
                    curl -s -X POST "https://api.telegram.org/bot${env.TELEGRAM_BOT_TOKEN}/sendMessage" \\
                        -d "chat_id=${env.TELEGRAM_CHAT_ID}" \\
                        --data-urlencode "text=${message}"
                """
            }
        }

        failure {
            script {
                def message = """【 알림 > Jenkins > ${env.PROJECT_NAME} 】

❌ 빌드 작업이 실패하였습니다.

커밋: ${env.GIT_COMMIT_SHORT}
빌드: #${env.BUILD_NUMBER}
시간: ${env.BUILD_TIMESTAMP}

${env.BUILD_URL}"""
                
                sh """
                    curl -s -X POST "https://api.telegram.org/bot${env.TELEGRAM_BOT_TOKEN}/sendMessage" \\
                        -d "chat_id=${env.TELEGRAM_CHAT_ID}" \\
                        --data-urlencode "text=${message}"
                """
            }
        }

        always {
            cleanWs()
        }

    }

}
