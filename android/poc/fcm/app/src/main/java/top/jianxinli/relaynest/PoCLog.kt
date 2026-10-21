package top.jianxinli.relaynest

import android.content.Context
import android.util.Log
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * PoC 共享事件日志：进程内单例，Activity 与 FCM 服务共用。
 * 约束（P02-01 验收）：写入这里的内容必须已脱敏，不含 token 全文与任何秘密。
 * 同步镜像到 logcat（tag=RelayNestFCM），便于测试侧经 adb 留证据。
 */
object PoCLog {
    private const val TAG = "RelayNestFCM"
    private val fmt = SimpleDateFormat("MM-dd HH:mm:ss.SSS", Locale.US)
    private val lines = mutableListOf<String>()
    private var listener: ((String) -> Unit)? = null

    /** 追加一行事件；text 必须已脱敏。 */
    @Synchronized
    fun log(text: String) {
        val line = "${fmt.format(Date())}  $text"
        lines.add(line)
        Log.i(TAG, line)
        listener?.invoke(line)
    }

    /** 注册 UI 刷新回调：Activity onResume 挂上、onPause 摘除，避免泄漏。 */
    @Synchronized
    fun setListener(l: ((String) -> Unit)?) {
        listener = l
    }

    @Synchronized
    fun snapshot(): String = lines.joinToString("\n")

    /** Token 脱敏形态：前 8 位 + 长度 + 后 4 位，足够比对又不可还原。 */
    fun maskToken(token: String): String =
        if (token.length <= 12) "***len=${token.length}***"
        else "${token.take(8)}…(len=${token.length})…${token.takeLast(4)}"
}

/**
 * Token 私有存储：全文只落应用私有目录（files/fcm_token.txt），
 * 供测试侧经 `adb shell run-as top.jianxinli.relaynest` 读取用于发送桩；
 * 不进日志、不进界面、不进报告。
 */
object TokenStore {
    private const val FILE = "fcm_token.txt"

    fun save(context: Context, token: String) {
        context.filesDir.resolve(FILE).writeText(token)
    }

    fun read(context: Context): String? =
        context.filesDir.resolve(FILE).takeIf { it.exists() }?.readText()
}
