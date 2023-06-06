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

        //stage('빌드') {
        //    steps {
                //sh "/usr/local/go/bin/go build"
        //    }
        //}

        //stage('배포') {
        //    steps {
                //sh '''
                //    sudo cp -f ./notify-server /usr/local/notify-server/
                //    sudo cp -f ./notify-server.sh /usr/local/notify-server/
                //    sudo cp -f ./notify-server-restart.sh /usr/local/notify-server/
                //    sudo cp -f ./secrets/notify-server.운영.json /usr/local/notify-server/notify-server.json

                //    sudo chown pi:staff /usr/local/notify-server/notify-server
                //    sudo chown pi:staff /usr/local/notify-server/notify-server.sh
                //    sudo chown pi:staff /usr/local/notify-server/notify-server-restart.sh
                //    sudo chown pi:staff /usr/local/notify-server/notify-server.json
                //'''
        //    }
        //}

        //stage('서버 재시작') {
        //    steps {
                // 경로를 이동하지 않고 서버를 재시작하게 되면 로그 파일의 생성 위치가
                // '/usr/local/notify-server/logs'에 생성되는게 아니라 Jenkins 작업 위치에 생성되게 되는데
                // 이때 'logs' 폴더가 존재하지 않으므로 서버 실행이 실패하게 된다.
                //sh '''
                //    cd /usr/local/notify-server
                //    sudo -u pi /usr/local/notify-server/notify-server-restart.sh
                //'''
        //    }
        //}

    }

    post {
        success {
            script {
                echo env.TELEGRAM_BOT_TOKEN
                
                // telegramSend(message: '【 알림 > Jenkins > ' + env.PROJECT_NAME + ' 】\n\n빌드 작업이 성공하였습니다.\n\n' + env.BUILD_URL, chatId=297396697)
                telegramSend(message: '【 알림 > Jenkins > 빌드 작업이 성공하였습니다.', chatId=297396697)
                sh '''
                    curl -s -X POST https://api.telegram.org/bot1550645495:AAGsszvqxsCTANl1nGkjY-26x3Vma4ARFNo/sendMessage -d chat_id=297396697 -d text="your message"
                '''
            }
        }
        failure {
            script {
                telegramSend(message: '【 알림 > Jenkins > ' + env.PROJECT_NAME + ' 】\n\n빌드 작업이 실패하였습니다.\n\n' + env.BUILD_URL, chatId=297396697)
            }
        }
        always {
            cleanWs()
        }
    }

}
