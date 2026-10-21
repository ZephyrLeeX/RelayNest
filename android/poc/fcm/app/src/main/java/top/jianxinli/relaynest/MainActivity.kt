package top.jianxinli.relaynest

import android.app.Activity
import android.graphics.Typeface
import android.os.Bundle
import android.widget.ScrollView
import android.widget.TextView
import com.google.firebase.messaging.FirebaseMessaging

/**
 * P02-01/02 最小界面：刻意不引入任何 UI 框架（Compose 属正式工程 P03-03 范围），
 * 单个等宽滚动日志视图，只显示脱敏注册状态、接收记录与补偿时序。
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
        PoCLog.log("Activity onResume（前台）")
        // 前台补偿：每次前台打开立即触发一次同步（T9.1）
        SyncJobs.enqueueNow(this, "app-open")
        render()
        PoCLog.setListener { runOnUiThread { render() } }
    }

    override fun onPause() {
        PoCLog.log("Activity onPause（离开前台）")
        PoCLog.setListener(null)
        super.onPause()
    }

    private fun render() {
        view.text = PoCLog.snapshot()
    }
}
