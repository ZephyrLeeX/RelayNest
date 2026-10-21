// AGP 9 内置 Kotlin：不 apply org.jetbrains.kotlin.android；
// jvmTarget 默认跟随下方 compileOptions.targetCompatibility（=17），无需单独设置
plugins {
    id("com.android.application")
    id("com.google.gms.google-services")
}

android {
    namespace = "top.jianxinli.relaynest"
    compileSdk = 36

    defaultConfig {
        applicationId = "top.jianxinli.relaynest"
        minSdk = 29
        targetSdk = 36
        versionCode = 1
        versionName = "0.0.1"
    }

    buildTypes {
        // PoC 不做混淆与压缩，保持可读可调试
        release {
            isMinifyEnabled = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    // 唯一功能依赖：FCM 客户端 SDK（锁定 25.1.3）
    implementation("com.google.firebase:firebase-messaging:25.1.3")
}
