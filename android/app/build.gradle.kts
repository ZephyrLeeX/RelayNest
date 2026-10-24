import org.gradle.api.configuration.BuildFeatures
import java.io.File
import javax.inject.Inject

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.compose.compiler)
}

dependencyLocking {
    lockAllConfigurations()
}

// 只检查外部配置和文件元数据，不读取 keystore；异常仅包含字段名和固定原因。
fun checkedReleaseSigningEnvironment(): Map<String, String> {
    val names =
        listOf(
            "RELAYNEST_ANDROID_KEYSTORE_PATH",
            "RELAYNEST_ANDROID_KEYSTORE_PASSWORD",
            "RELAYNEST_ANDROID_KEY_ALIAS",
            "RELAYNEST_ANDROID_KEY_PASSWORD",
        )
    val values = names.associateWith { System.getenv(it).orEmpty() }
    val missing = names.filter { values.getValue(it).isBlank() }
    if (missing.isNotEmpty()) {
        throw GradleException("正式签名配置缺少字段：${missing.joinToString()}（值已隐藏）")
    }
    val referenceValid =
        try {
            val reference = File(values.getValue("RELAYNEST_ANDROID_KEYSTORE_PATH"))
            reference.exists() && reference.isFile && reference.canRead()
        } catch (_: SecurityException) {
            false
        }
    if (!referenceValid) {
        throw GradleException("正式签名文件引用无效：要求存在、普通文件且可读（引用已隐藏）")
    }
    return values
}

val releaseSigningRequested = providers.gradleProperty("relaynestReleaseSigning").orNull == "true"

abstract class SigningBuildFeatures {
    @get:Inject
    abstract val buildFeatures: BuildFeatures
}

// 签名模型包含口令，不允许将其序列化至配置缓存。
val releaseSigningEnvironment =
    if (releaseSigningRequested) {
        if (objects
                .newInstance<SigningBuildFeatures>()
                .buildFeatures.configurationCache.requested
                .get()
        ) {
            throw GradleException("正式签名请求必须使用 --no-configuration-cache")
        }
        checkedReleaseSigningEnvironment()
    } else {
        null
    }

tasks.register("checkReleaseSigningConfig") {
    group = "verification"
    description = "仅检查正式签名配置完整性及文件引用元数据，不验证密钥内容或执行签名。"
    notCompatibleWithConfigurationCache("诊断不缓存外部签名配置")
    doLast {
        checkedReleaseSigningEnvironment()
        logger.lifecycle("正式签名配置完整，文件引用为可读普通文件；未读取密钥内容，未验证口令、别名或签名。")
    }
}

android {
    namespace = "top.jianxinli.relaynest"
    compileSdk = 36
    buildToolsVersion = "36.0.0"

    defaultConfig {
        applicationId = "top.jianxinli.relaynest"
        minSdk = 29
        targetSdk = 36
        versionCode = 1
        versionName = "0.0.1"
    }

    buildFeatures {
        compose = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    val externalReleaseSigning =
        releaseSigningEnvironment?.let { values ->
            signingConfigs.create("externalRelease") {
                storeFile = File(values.getValue("RELAYNEST_ANDROID_KEYSTORE_PATH"))
                storePassword = values.getValue("RELAYNEST_ANDROID_KEYSTORE_PASSWORD")
                keyAlias = values.getValue("RELAYNEST_ANDROID_KEY_ALIAS")
                keyPassword = values.getValue("RELAYNEST_ANDROID_KEY_PASSWORD")
            }
        }

    // 只有显式请求才接入正式身份；缺失配置时失败，绝不回退 debug 签名。
    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = externalReleaseSigning
        }
    }
}

dependencies {
    implementation(libs.okhttp)
    implementation(libs.coroutines.core)
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
    implementation("androidx.lifecycle:lifecycle-viewmodel-ktx:2.9.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.9.0")
    // 新客户端的旧版 stdlib 兼容桥沿用仓库已锁定版本，不引入另一组 Kotlin 版本。
    constraints {
        implementation(libs.kotlin.stdlib.jdk7)
        implementation(libs.kotlin.stdlib.jdk8)
    }
    testImplementation(libs.junit)
    testImplementation(libs.mockwebserver)
    testImplementation(libs.okhttp.tls)
    // Android 已提供 org.json；本地 JVM 测试使用实际解析器而非 android.jar 空桩。
    testImplementation(libs.json.test)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.foundation)
    implementation(libs.androidx.compose.material3)
}
