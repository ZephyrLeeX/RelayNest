package top.jianxinli.relaynest

import android.app.Application
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import java.util.concurrent.TimeUnit

/**
 * P02-02：进程级初始化——日志文件、屏幕亮灭时序记录、周期补偿任务登记。
 * 不注册任何常驻前台服务（验收项：不增加常驻服务）。
 */
class PoCApp : Application() {

    override fun onCreate() {
        super.onCreate()
        PoCLog.init(this)
        PoCLog.log("App 进程启动（pid=${android.os.Process.myPid()}）")

        // 屏幕亮灭广播：进程存活期间记录锁屏时序（P02-02 验收：记录真实时间）
        val screenReceiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context, intent: Intent) {
                when (intent.action) {
                    Intent.ACTION_SCREEN_OFF -> PoCLog.log("SCREEN_OFF（灭屏/锁屏）")
                    Intent.ACTION_SCREEN_ON -> PoCLog.log("SCREEN_ON（亮屏）")
                }
            }
        }
        registerReceiver(
            screenReceiver,
            IntentFilter().apply {
                addAction(Intent.ACTION_SCREEN_OFF)
                addAction(Intent.ACTION_SCREEN_ON)
            },
        )

        // 周期补偿：30 分钟 + 网络可用 + 唯一任务（T9.1 初始值；系统最短实际 15 分钟）
        val periodic = PeriodicWorkRequestBuilder<SyncWorker>(30, TimeUnit.MINUTES)
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build()
            )
            .setInputData(androidx.work.workDataOf(SyncWorker.KEY_TRIGGER to "periodic"))
            .build()
        WorkManager.getInstance(this).enqueueUniquePeriodicWork(
            "poc-periodic-sync",
            ExistingPeriodicWorkPolicy.KEEP,
            periodic,
        )
        PoCLog.log("已登记周期补偿任务（30min，网络约束，KEEP）")
    }
}
