// 根构建脚本：只声明插件版本（锁定兼容 SDK，版本来源为 Google Maven / Maven Central 元数据，2026-10-21）
plugins {
    id("com.android.application") version "9.4.1" apply false
    id("com.google.gms.google-services") version "4.5.0" apply false
}
