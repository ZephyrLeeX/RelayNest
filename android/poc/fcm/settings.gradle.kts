// P02-01 隔离 PoC 工程：独立 settings，不与仓库其他部分共享 Gradle 配置
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}
rootProject.name = "relaynest-fcm-poc"
include(":app")
