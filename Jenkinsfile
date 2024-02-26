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