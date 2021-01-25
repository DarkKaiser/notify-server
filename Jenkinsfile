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

        stage('체크아웃') {
            steps {
                checkout([$class: 'GitSCM', branches: [[name: '*/master']], doGenerateSubmoduleConfigurations: false, extensions: [], submoduleCfg: [], userRemoteConfigs: [[credentialsId: 'github-darkkaiser-credentials', url: 'https://github.com/DarkKaiser/notify-server.git']]])
            }
        }

        stage('빌드') {
            steps {
                sh "/usr/local/go/bin/go build"
            }
        }

        stage('배포') {
            steps {
                sh '''
                    sudo cp -f ./notify-server /usr/local/notify-server/
                    sudo cp -f ./notify-server.sh /usr/local/notify-server/
                    sudo cp -f ./notify-server-restart.sh /usr/local/notify-server/
                    sudo cp -f ./notify-server.운영.json /usr/local/notify-server/notify-server.json

                    sudo chown pi:staff /usr/local/notify-server/notify-server
                    sudo chown pi:staff /usr/local/notify-server/notify-server.sh
                    sudo chown pi:staff /usr/local/notify-server/notify-server-restart.sh
                    sudo chown pi:staff /usr/local/notify-server/notify-server.json
                '''
            }
        }

        stage('서버 재시작') {
            steps {
                // 현재의 경로를 이동시켜 주지 않고 실행하게 되면 로그파일의 생성위치가 /usr/local/notify-server/logs에 생성되는게 아니라
                // Jenkins 작업 위치에 생성되게 되는데 이때 logs 폴더가 존재하지 않아서 서버 실행이 실패하게 된다.
                sh '''
                    cd /usr/local/notify-server
                    sudo -u pi /usr/local/notify-server/notify-server-restart.sh
                '''
            }
        }

    }

    post {
        success {
            script {
                telegramSend(message: '【 알림 > Jenkins > ' + env.PROJECT_NAME + ' 】\n\n빌드 작업이 성공하였습니다.\n\n' + env.BUILD_URL)
            }
        }
        failure {
            script {
                telegramSend(message: '【 알림 > Jenkins > ' + env.PROJECT_NAME + ' 】\n\n빌드 작업이 실패하였습니다.\n\n' + env.BUILD_URL)
            }
        }
    }

}
