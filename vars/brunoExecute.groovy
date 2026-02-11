import com.sap.piper.ConfigurationHelper
import groovy.transform.Field
import com.sap.piper.GitUtils

import static com.sap.piper.Prerequisites.checkScript

@Field String STEP_NAME = getClass().getName()
@Field String METADATA_FILE = 'metadata/brunoExecute.yaml'

@Field Set CONFIG_KEYS = [
    /**
     * Define an additional repository where the test implementation is located.
     * For protected repositories the `testRepository` needs to contain the ssh git url.
     */
    'testRepository',
    /**
     * Only if `testRepository` is provided: Branch of testRepository, defaults to master.
     */
    'gitBranch',
    /**
     * Only if `testRepository` is provided: Credentials for a protected testRepository
     * @possibleValues Jenkins credentials id
     */
    'gitSshKeyCredentialsId',
]

void call(Map parameters = [:]) {
    final script = checkScript(this, parameters) ?: this
    String stageName = parameters.stageName ?: env.STAGE_NAME
    Map config = ConfigurationHelper.newInstance(this)
        .loadStepDefaults([:], stageName)
        .mixinGeneralConfig(script.commonPipelineEnvironment, CONFIG_KEYS)
        .mixinStepConfig(script.commonPipelineEnvironment, CONFIG_KEYS)
        .mixinStageConfig(script.commonPipelineEnvironment, stageName, CONFIG_KEYS)
        .mixin(parameters, CONFIG_KEYS)
        .use()

    if (parameters.testRepository || config.testRepository) {
        parameters.stashContent = [GitUtils.handleTestRepository(this, [
            gitBranch: config.gitBranch,
            gitSshKeyCredentialsId: config.gitSshKeyCredentialsId,
            testRepository: config.testRepository
        ])]
    }

    piperExecuteBin(parameters, STEP_NAME, METADATA_FILE, [])
}
