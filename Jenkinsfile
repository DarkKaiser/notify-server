pipeline {

    agent any

    environment {
        PROJECT_NAME = "NotifyServer"
    }

    stages {

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
            }
        }

        stage('테스트 및 코드 품질 검사') {
            steps {
                // Dockerfile의 builder 스테이지에서 테스트와 린트(golangci-lint)가 모두 실행됩니다.
                // 이를 통해 Jenkins 환경(DinD)에서의 볼륨 마운트 문제를 해결하고, 일관된 검사 환경을 보장합니다.
                sh 'docker build --target builder -t notify-server:test .'
                sh 'docker run --rm notify-server:test go test ./... -v'
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
                sh "docker build -t darkkaiser/notify-server ."
            }
        }

        stage('도커 컨테이너 실행') {
            steps {
                sh '''
                    docker ps -q --filter name=notify-server | grep -q . && docker container stop notify-server && docker container rm notify-server

                    docker run -d --name notify-server \
                                  -e TZ=Asia/Seoul \
                                  -v /usr/local/docker/notify-server:/usr/local/app \
                                  -v /usr/local/docker/nginx-proxy-manager/letsencrypt:/etc/letsencrypt:ro \
                                  -p 2443:2443 \
                                  --restart="always" \
                                  darkkaiser/notify-server
                '''
            }
        }

        stage('도커 이미지 정리') {
            steps {
                sh 'docker images -qf dangling=true | xargs -I{} docker rmi {}'
            }
        }
        
    }

    post {

        success {
            script {
                sh "curl -s -X POST https://api.telegram.org/bot${env.TELEGRAM_BOT_TOKEN}/sendMessage -d chat_id=${env.TELEGRAM_CHAT_ID} -d text='【 알림 > Jenkins > ${env.PROJECT_NAME} 】\n\n빌드 작업이 성공하였습니다.\n\n${env.BUILD_URL}'"
            }
        }

        failure {
            script {
                sh "curl -s -X POST https://api.telegram.org/bot${env.TELEGRAM_BOT_TOKEN}/sendMessage -d chat_id=${env.TELEGRAM_CHAT_ID} -d text='【 알림 > Jenkins > ${env.PROJECT_NAME} 】\n\n빌드 작업이 실패하였습니다.\n\n${env.BUILD_URL}'"
            }
        }

        always {
            cleanWs()
        }

    }

}