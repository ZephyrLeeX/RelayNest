package top.jianxinli.relaynest

import android.content.Context
import android.util.Log
import java.io.FileWriter
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * PoC 共享事件日志：进程内单例，Activity 与 FCM 服务共用。
 * 约束（P02-01/02 验收）：写入这里的内容必须已脱敏，不含 token 全文与任何秘密。
 * 三路输出：内存（UI 显示）、logcat（tag=RelayNestFCM，adb 侧证据）、
 * 应用私有文件 poc_log.txt（跨进程重启的证据时间线，P02-02 遗留项 2 的修复）。
 */
object PoCLog {
    private const val TAG = "RelayNestFCM"
    private const val FILE = "poc_log.txt"
    private const val MAX_LINES = 400

    private val fmt = SimpleDateFormat("MM-dd HH:mm:ss.SSS", Locale.US)
    private val lines = mutableListOf<String>()
    private var listener: ((String) -> Unit)? = null
    private var writer: FileWriter? = null

    /** 由 Application 初始化：打开追加文件并载入历史（最近 MAX_LINES 行）供界面连续展示。 */
    @Synchronized
    fun init(context: Context) {
        val file = context.filesDir.resolve(FILE)
        if (file.exists()) {
            lines.addAll(file.readLines().takeLast(MAX_LINES))
        }
        writer = FileWriter(file, true)
    }

    /** 追加一行事件；text 必须已脱敏。 */
    @Synchronized
    fun log(text: String) {
        val line = "${fmt.format(Date())}  $text"
        lines.add(line)
        if (lines.size > MAX_LINES) lines.removeAt(0)
        Log.i(TAG, line)
        writer?.let {
            it.write(line + "\n")
            it.flush()
        }
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
