timestamps {
    node () {
        dir ("../src/github.com/Conservify/goridium") {
            stage ('git') {
                checkout([$class: 'GitSCM', branches: [[name: '*/master']], userRemoteConfigs: [[url: 'https://github.com/Conservify/goridium.git']]])
            }

            stage ('build') {
            }
        }
    }
}
