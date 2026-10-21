package top.jianxinli.relaynest

import android.app.Activity
import android.graphics.Typeface
import android.os.Bundle
import android.widget.ScrollView
import android.widget.TextView
import com.google.firebase.messaging.FirebaseMessaging

/**
 * P02-01 最小注册界面：刻意不引入任何 UI 框架（Compose 属正式工程 P03-03 范围），
 * 单个等宽滚动日志视图，只显示脱敏注册状态与收到的 data 提示。
 */
class MainActivity : Activity() {
    private lateinit var view: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        view = TextView(this).apply {
            typeface = Typeface.MONOSPACE
            textSize = 12f
            setPadding(24, 24, 24, 24)
            setTextIsSelectable(true)
        }
        setContentView(ScrollView(this).apply { addView(view) })
        PoCLog.log("App 启动，开始 FCM 注册")

        // 注册并获取当前 token：成功后全文只写入私有文件，日志仅记脱敏形态
        FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            if (task.isSuccessful) {
                val token = task.result
                TokenStore.save(this, token)
                PoCLog.log("token 获取成功: ${PoCLog.maskToken(token)}")
            } else {
                PoCLog.log(
                    "token 获取失败: ${task.exception?.javaClass?.simpleName}: ${task.exception?.message}"
                )
            }
        }
    }

    override fun onResume() {
        super.onResume()
        render()
        PoCLog.setListener { runOnUiThread { render() } }
    }

    override fun onPause() {
        PoCLog.setListener(null)
        super.onPause()
    }

    private fun render() {
        view.text = PoCLog.snapshot()
    }
}
