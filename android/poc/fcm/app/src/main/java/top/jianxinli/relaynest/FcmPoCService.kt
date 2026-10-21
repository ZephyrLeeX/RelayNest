package top.jianxinli.relaynest

import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * P02-01 数据提示接收服务：只处理 data-only 载荷，不展示系统通知。
 * 载荷内容为测试自造的 type/deployment_id/watermark（T9.1 形态），无正文、无 URL、无凭据。
 */
class FcmPoCService : FirebaseMessagingService() {

    override fun onNewToken(token: String) {
        // Token 轮换：更新私有文件并记录脱敏形态
        TokenStore.save(this, token)
        PoCLog.log("onNewToken: ${PoCLog.maskToken(token)}")
    }

    override fun onMessageReceived(message: RemoteMessage) {
        // 记录消息 ID、优先级（折算后/原始）、发送时间与 data 键值，供 PoC 时序分析
        val data = message.data.entries.joinToString(", ") { "${it.key}=${it.value}" }
        PoCLog.log(
            "onMessageReceived id=${message.messageId} " +
                "prio=${message.priority}/${message.originalPriority} " +
                "sent=${message.sentTime} data=[$data]"
        )
    }

    override fun onDeletedMessages() {
        // 服务端丢弃消息的信号：P02-02 补偿机制的依据，先如实记录
        PoCLog.log("onDeletedMessages: 服务端丢弃了待投递消息")
    }
}
