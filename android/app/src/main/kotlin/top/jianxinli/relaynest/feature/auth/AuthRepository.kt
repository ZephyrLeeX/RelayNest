package top.jianxinli.relaynest.feature.auth

import android.content.Context
import java.io.File
import java.util.UUID
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import top.jianxinli.relaynest.core.network.*
import top.jianxinli.relaynest.core.security.*

enum class AuthPhase { UNAUTHENTICATED, LOGGING_IN, RESTORING, AUTHENTICATED, EXPIRED, RETRYABLE_ERROR, ERROR }

/** 状态中只放可展示元数据；凭据不流入 Compose 或保存状态。 */
data class AuthState(
    val phase: AuthPhase = AuthPhase.UNAUTHENTICATED,
    val endpoint: HubEndpoint? = null,
    val session: HubSession? = null,
    val message: String = "请配置 Bot Hub HTTPS 地址",
    val canRetryRestore: Boolean = false,
)

internal interface AuthStorage {
    fun endpoint(): HubEndpoint?
    fun remember(endpoint: HubEndpoint?)
    fun begin(endpoint: HubEndpoint): SessionAttempt
    fun restore(attempt: SessionAttempt): StoredSession?
    fun save(attempt: SessionAttempt, login: HubLogin): Boolean
    fun isCurrent(attempt: SessionAttempt): Boolean
    fun clear()
    fun clear(attempt: SessionAttempt): Boolean
    fun deviceId(): UUID
}

internal interface AuthRemote {
    suspend fun login(endpoint: HubEndpoint, request: HubLoginRequest): HubResult<HubLogin>
    suspend fun session(endpoint: HubEndpoint, credential: HubCredential): HubResult<HubSession>
    suspend fun logout(endpoint: HubEndpoint, credential: HubCredential): HubResult<Unit>
}

class AuthRepository internal constructor(
    private val storage: AuthStorage,
    private val remote: AuthRemote,
    private val io: CoroutineDispatcher = Dispatchers.IO,
) {
    constructor(context: Context) : this(AndroidAuthStorage(context.applicationContext), ClientAuthRemote())

    private val guard = Any()
    private val disk = Mutex()
    private var generation = 0L
    private var credential: HubCredential? = null
    private val mutableState = MutableStateFlow(AuthState())
    val state = mutableState.asStateFlow()

    // 先撤下内存身份，再异步处理磁盘；迟到网络结果和旧磁盘操作都必须核对代次。
    private fun begin(phase: AuthPhase, endpoint: HubEndpoint?, message: String): Long = synchronized(guard) {
        generation++
        credential = null
        mutableState.value = AuthState(phase, endpoint, message = message)
        generation
    }

    private fun current(id: Long): Boolean = synchronized(guard) { generation == id }
    private fun publish(id: Long, state: AuthState, secret: HubCredential? = null) = synchronized(guard) {
        if (generation == id) {
            credential = secret
            mutableState.value = state
        }
    }

    private suspend fun <T> onDisk(block: () -> T): T = withContext(io) { disk.withLock { block() } }

    suspend fun initialize() {
        val id = begin(AuthPhase.RESTORING, null, "正在读取本地会话")
        guarded(id) {
            val endpoint = onDisk { if (current(id)) storage.endpoint() else null }
            if (!current(id)) return@guarded
            if (endpoint == null) publish(id, AuthState()) else restore(id, endpoint)
        }
    }

    suspend fun connect(address: String) {
        val id = begin(AuthPhase.RESTORING, null, "正在校验地址")
        guarded(id) {
            val endpoint = try { HubEndpoint.parse(address) } catch (_: IllegalArgumentException) {
                publish(id, AuthState(message = "请输入有效的 HTTPS 部署基础地址"))
                return@guarded
            }
            onDisk {
                if (current(id)) {
                    // 显式选择地址属于身份边界，旧凭据不得跟随新的配置。
                    storage.clear()
                    storage.remember(endpoint)
                }
            }
            if (current(id)) publish(id, AuthState(endpoint = endpoint, message = "请输入 Hub 账号和密码"))
        }
    }

    suspend fun retryRestore() {
        if (state.value.phase != AuthPhase.AUTHENTICATED && !state.value.canRetryRestore) return
        val endpoint = state.value.endpoint ?: return
        val id = begin(AuthPhase.RESTORING, endpoint, "正在向 Hub 验证会话")
        guarded(id) { restore(id, endpoint) }
    }

    private suspend fun restore(id: Long, endpoint: HubEndpoint) {
        publish(id, AuthState(AuthPhase.RESTORING, endpoint, message = "正在向 Hub 验证会话"))
        val pair = onDisk {
            if (!current(id)) null else {
                val attempt = storage.begin(endpoint)
                attempt to storage.restore(attempt)
            }
        } ?: return
        val (attempt, stored) = pair
        if (!current(id)) return
        if (stored == null) {
            publish(id, AuthState(endpoint = endpoint, message = "没有可恢复的会话，请登录"))
            return
        }
        when (val result = remote.session(endpoint, stored.credential)) {
            is HubResult.Failure -> failure(id, endpoint, attempt, result, restoring = true)
            is HubResult.Success -> {
                if (!current(id)) return
                // GET 元数据必须仍属于保存的会话及本安装，不能把另一身份当成恢复成功。
                val valid = result.value.id == stored.sessionId && result.value.deviceId == onDisk { storage.deviceId() }
                if (!valid) {
                    onDisk { if (current(id)) storage.clear(attempt) }
                    publish(id, AuthState(AuthPhase.EXPIRED, endpoint, message = "会话身份不匹配，请重新登录"))
                } else if (onDisk { current(id) && storage.isCurrent(attempt) }) {
                    publish(id, AuthState(AuthPhase.AUTHENTICATED, endpoint, result.value, "已登录 Hub"), stored.credential)
                }
            }
        }
    }

    suspend fun login(username: String, password: String, deviceName: String) {
        val endpoint = state.value.endpoint ?: return
        val id = begin(AuthPhase.LOGGING_IN, endpoint, "正在登录 Hub")
        guarded(id) {
            val prepared = onDisk {
                if (!current(id)) null else {
                    storage.clear()
                    storage.begin(endpoint) to storage.deviceId()
                }
            } ?: return@guarded
            val (attempt, deviceId) = prepared
            if (!current(id)) return@guarded
            if (username.isBlank() || password.isEmpty() || deviceName.isBlank()) {
                publish(id, AuthState(endpoint = endpoint, message = "请填写账号、密码和设备名称"))
                return@guarded
            }
            when (val result = remote.login(endpoint, HubLoginRequest(username, password, deviceName, deviceId))) {
                is HubResult.Failure -> failure(id, endpoint, attempt, result, restoring = false)
                is HubResult.Success -> {
                    val saved = onDisk { current(id) && storage.save(attempt, result.value) && storage.isCurrent(attempt) }
                    if (saved) publish(id, AuthState(AuthPhase.AUTHENTICATED, endpoint, result.value.session, "已登录 Hub"), result.value.credential)
                    else publish(id, AuthState(AuthPhase.EXPIRED, endpoint, message = "会话操作已失效，请重新登录"))
                }
            }
        }
    }

    suspend fun changeAddress() {
        val id = begin(AuthPhase.UNAUTHENTICATED, null, "正在清除旧地址会话")
        // 身份终止的本地删除不能被 UI 请求取消；下一个操作仍按代次串行。
        withContext(NonCancellable) {
            guarded(id) {
                onDisk { if (current(id)) { storage.clear(); storage.remember(null) } }
                publish(id, AuthState())
            }
        }
    }

    suspend fun logout() {
        val old = synchronized(guard) { state.value.endpoint to credential }
        val id = begin(AuthPhase.UNAUTHENTICATED, old.first, "正在结束本地会话并确认服务端注销")
        withContext(NonCancellable) {
            guarded(id) {
                // 清理失败仍尽力注销远端，但绝不声称本地持久会话已删除。
                val cleared = try {
                    onDisk { if (current(id)) storage.clear() }
                    true
                } catch (_: Exception) { false }
                val endpoint = old.first
                val secret = old.second
                val message = if (endpoint == null || secret == null) {
                    "本地会话已结束；未确认服务端注销"
                } else {
                    val result = remote.logout(endpoint, secret)
                    val confirmed = result is HubResult.Success || (result is HubResult.Failure && result.kind == HubError.UNAUTHORIZED)
                    if (confirmed) "本地会话已结束，服务端会话已失效" else "本地会话已结束；服务端注销未确认，可重新登录"
                }
                if (!cleared) publish(id, AuthState(AuthPhase.ERROR, endpoint, message = "本地安全存储清理未确认，请重试清理后退出"))
                else publish(id, AuthState(endpoint = endpoint, message = message))
            }
        }
    }

    private suspend fun failure(id: Long, endpoint: HubEndpoint, attempt: SessionAttempt, error: HubResult.Failure, restoring: Boolean) {
        if (!current(id)) return
        if (error.kind == HubError.UNAUTHORIZED) {
            onDisk { if (current(id)) storage.clear(attempt) }
            publish(id, AuthState(if (restoring) AuthPhase.EXPIRED else AuthPhase.UNAUTHENTICATED, endpoint,
                message = if (restoring) "会话已过期或被撤销，请重新登录" else "账号或密码错误，请重新输入"))
        } else {
            val retryable = error.kind in setOf(HubError.NETWORK, HubError.RATE_LIMITED, HubError.SERVER)
            val message = when (error.kind) {
                HubError.NETWORK -> "网络不可用，请稍后重试"
                HubError.RATE_LIMITED -> "请求过于频繁，请稍后重试" + (error.retryAfterSeconds?.let { "（建议等待 ${it} 秒）" } ?: "")
                HubError.SERVER -> "Hub 暂时不可用，请稍后重试"
                HubError.TLS -> "无法验证 Hub 安全连接，请检查地址与证书"
                else -> "Hub 响应无效，请检查部署地址后重试"
            }
            publish(id, AuthState(if (retryable) AuthPhase.RETRYABLE_ERROR else AuthPhase.ERROR, endpoint,
                message = message, canRetryRestore = restoring))
        }
    }

    private suspend fun guarded(id: Long, block: suspend () -> Unit) {
        try { block() } catch (cancelled: CancellationException) {
            throw cancelled
        } catch (_: Exception) {
            // 不展示原始异常；保存/删除失败不能显示已认证或已完成持久注销。
            publish(id, AuthState(AuthPhase.ERROR, state.value.endpoint, message = "本地安全存储操作失败，请重试；会话清理未确认"))
        }
    }
}

private class AndroidAuthStorage(private val context: Context) : AuthStorage {
    // noBackupFilesDir/canonicalFile 也可能访问文件系统，因此构造依赖延迟到首个 IO 操作。
    private val sessions by lazy { SessionStore(context) }
    private val identity by lazy { InstallationIdentity(context) }
    // 这里只保存非凭据地址；与密文会话一样排除在备份之外，所有访问均在 IO 调度器。
    private val address by lazy { HubAddressFile(File(context.noBackupFilesDir, "hub-address.v1")) }
    override fun endpoint() = address.read()
    override fun remember(endpoint: HubEndpoint?) = address.write(endpoint)
    override fun begin(endpoint: HubEndpoint) = sessions.begin(endpoint)
    override fun restore(attempt: SessionAttempt) = sessions.restore(attempt)
    override fun save(attempt: SessionAttempt, login: HubLogin) = sessions.save(attempt, login)
    override fun isCurrent(attempt: SessionAttempt) = sessions.isCurrent(attempt)
    override fun clear() = sessions.clear()
    override fun clear(attempt: SessionAttempt) = sessions.clear(attempt)
    override fun deviceId() = identity.getOrCreate()
}

/** 非凭据配置复用已存在的原子写工具；坏地址删除后要求重新配置，不猜测部署。 */
internal class HubAddressFile(private val file: File) {
    fun read(): HubEndpoint? {
        if (!file.exists()) return null
        return try {
            require(file.length() in 1..8192)
            HubEndpoint.parse(file.readText())
        } catch (_: IllegalArgumentException) {
            write(null)
            null
        }
    }
    fun write(endpoint: HubEndpoint?) {
        if (endpoint == null) check(!file.exists() || file.delete())
        else atomicWrite(file, endpoint.bindingKey.toByteArray(Charsets.UTF_8))
    }
}

private class ClientAuthRemote : AuthRemote {
    private val client = HubClient()
    override suspend fun login(endpoint: HubEndpoint, request: HubLoginRequest) = client.login(endpoint, request)
    override suspend fun session(endpoint: HubEndpoint, credential: HubCredential) = client.session(endpoint, credential)
    override suspend fun logout(endpoint: HubEndpoint, credential: HubCredential) = client.logout(endpoint, credential)
}
