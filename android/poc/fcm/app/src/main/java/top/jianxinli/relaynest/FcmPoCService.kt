package top.jianxinli.relaynest

import android.os.PowerManager
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * P02-01/02 数据提示接收服务：只处理 data-only 载荷，不展示系统通知。
 * 载荷内容为测试自造的 type/deployment_id/watermark（T9.1 形态），无正文、无 URL、无凭据。
 */
class FcmPoCService : FirebaseMessagingService() {

    override fun onNewToken(token: String) {
        // Token 轮换：更新私有文件并记录脱敏形态
        TokenStore.save(this, token)
        PoCLog.log("onNewToken: ${PoCLog.maskToken(token)}")
    }

    override fun onMessageReceived(message: RemoteMessage) {
        // 记录服务端 sentTime 与本地接收 epoch（时钟同源，差值即真实投递延迟）、
        // 到达时的屏幕与 Doze 状态，以及 data 键值，供后台时序分析
        val pm = getSystemService(PowerManager::class.java)
        val data = message.data.entries.joinToString(", ") { "${it.key}=${it.value}" }
        PoCLog.log(
            "onMessageReceived id=${message.messageId} " +
                "prio=${message.priority}/${message.originalPriority} " +
                "sent=${message.sentTime} recv=${System.currentTimeMillis()} " +
                "screenOn=${pm.isInteractive} doze=${pm.isDeviceIdleMode} " +
                "data=[$data]"
        )
    }

    override fun onDeletedMessages() {
        // 服务端丢弃消息的信号：触发补偿同步（P02-02 前台/后台补偿的一部分）
        PoCLog.log("onDeletedMessages: 服务端丢弃了待投递消息，触发补偿")
        SyncJobs.enqueueNow(this, "onDeletedMessages")
    }
}
