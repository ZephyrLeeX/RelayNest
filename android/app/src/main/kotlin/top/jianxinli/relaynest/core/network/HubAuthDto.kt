package top.jianxinli.relaynest.core.network

import java.time.Instant
import java.util.UUID
import org.json.JSONObject
import org.json.JSONTokener

// 含凭据的对象不使用 data class，避免自动 toString/copy 泄露密码或 Token。
class HubLoginRequest(
    private val username: String,
    private val password: String,
    private val deviceName: String,
    val deviceId: UUID,
) {
    init { require(username.isNotEmpty() && password.isNotEmpty() && deviceName.isNotEmpty()) }
    internal fun json(): String = JSONObject()
        .put("username", username).put("password", password).put("client_type", "android")
        .put("device_name", deviceName).put("device_id", deviceId.toString()).toString()
}

class HubCredential(val endpoint: HubEndpoint, internal val token: String) {
    init { require(token.matches(Regex("[A-Za-z0-9._~+/-]+=*"))) { "Bearer 格式无效" } }
}

data class HubSession(
    val id: UUID,
    val administratorUsername: String,
    val deviceId: UUID,
    val deviceName: String,
    val idleExpiresAt: Instant,
    val absoluteExpiresAt: Instant,
)

class HubLogin(val session: HubSession, val credential: HubCredential)

internal object HubAuthDto {
    fun root(body: String): JSONObject {
        val tokener = JSONTokener(body)
        val root = tokener.nextValue() as? JSONObject ?: error("响应必须为对象")
        require(tokener.nextClean() == '\u0000')
        return root
    }

    fun session(root: JSONObject): HubSession {
        val s = root.getJSONObject("session")
        require(text(s, "client_type") == "android")
        val admin = s.getJSONObject("administrator")
        return HubSession(
            uuid(s, "id"), text(admin, "username"), uuid(s, "device_id"),
            text(s, "device_name"), Instant.parse(text(s, "idle_expires_at")),
            Instant.parse(text(s, "absolute_expires_at")),
        )
    }

    fun text(obj: JSONObject, key: String): String =
        (obj.get(key) as? String)?.also { require(it.isNotEmpty()) } ?: error("响应字段类型无效")

    private fun uuid(obj: JSONObject, key: String): UUID {
        val value = text(obj, key)
        require(value.matches(Regex("[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}")))
        return UUID.fromString(value)
    }
}
