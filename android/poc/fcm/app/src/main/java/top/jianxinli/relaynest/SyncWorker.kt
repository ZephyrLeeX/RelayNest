package top.jianxinli.relaynest

import android.content.Context
import android.os.PowerManager
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import androidx.work.workDataOf

/**
 * P02-02 补偿同步任务：PoC 的“同步”为模拟动作（记录时间与触发来源），
 * 真实增量同步协议属 P08-06；本卡验证的是“推送失败/未达时补偿仍会运行”。
 */
class SyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val pm = applicationContext.getSystemService(PowerManager::class.java)
        val trigger = inputData.getString(KEY_TRIGGER) ?: "unknown"
        PoCLog.log(
            "SyncWork 运行 trigger=$trigger screenOn=${pm.isInteractive} doze=${pm.isDeviceIdleMode}"
        )
        // 记录本次同步时间，作为“重开仍同步”的可核查证据
        applicationContext.filesDir.resolve("last_sync.txt")
            .writeText("epoch=${System.currentTimeMillis()} trigger=$trigger")
        PoCLog.log("SyncWork 完成 trigger=$trigger")
        return Result.success()
    }

    companion object {
        const val KEY_TRIGGER = "trigger"
    }
}

/** 统一的补偿任务入口：唯一任务名 + KEEP，避免堆积；网络约束（T9.1）。 */
object SyncJobs {
    private const val UNIQUE_NOW = "poc-sync-now"

    fun enqueueNow(context: Context, trigger: String) {
        val request = OneTimeWorkRequestBuilder<SyncWorker>()
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build()
            )
            .setInputData(workDataOf(SyncWorker.KEY_TRIGGER to trigger))
            .build()
        WorkManager.getInstance(context)
            .enqueueUniqueWork(UNIQUE_NOW, ExistingWorkPolicy.KEEP, request)
        PoCLog.log("已登记即时补偿任务 trigger=$trigger（KEEP）")
    }
}
