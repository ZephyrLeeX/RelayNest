package top.jianxinli.relaynest.core.security

import android.content.Context
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.File
import java.nio.file.Files
import java.util.UUID
import top.jianxinli.relaynest.core.network.HubCredential
import top.jianxinli.relaynest.core.network.HubEndpoint
import top.jianxinli.relaynest.core.network.HubLogin

/** 只保存 Bearer、目的地址和 session.id；恢复后由 S03 向服务端重新验证会话。需在 IO 线程调用。 */
class SessionStore internal constructor(directory: File, private val cipher: SessionCipher) {
    constructor(context: Context) : this(context.noBackupFilesDir, SessionCipher())

    private val file = File(directory, "hub-session.v1").canonicalFile
    private val state = synchronized(SecurityStorageLock) { states.getOrPut(file.path) { GenerationState() } }

    /** 登录/恢复/切换地址前创建票据。每次调用使所有旧操作失效，同地址重试也适用。 */
    fun begin(endpoint: HubEndpoint): SessionAttempt = synchronized(SecurityStorageLock) {
        state.generation++
        if (state.endpoint != null && state.endpoint != endpoint) erase()
        state.endpoint = endpoint
        SessionAttempt(state, state.generation, endpoint)
    }

    fun isCurrent(attempt: SessionAttempt): Boolean = synchronized(SecurityStorageLock) { current(attempt) }

    fun restore(attempt: SessionAttempt): StoredSession? = synchronized(SecurityStorageLock) {
        if (!current(attempt) || !file.exists()) return@synchronized null
        try {
            require(file.length() in 1..MAX_RECORD_SIZE.toLong())
            DataInputStream(ByteArrayInputStream(file.readBytes())).use { input ->
                require(input.readInt() == VERSION)
                val endpoint = HubEndpoint.parse(input.readUTF())
                val sessionId = UUID.fromString(input.readUTF())
                require(endpoint == attempt.endpoint)
                val iv = ByteArray(input.readInt().also { require(it == 12) }).also { input.readFully(it) }
                val size = input.readInt().also { require(it in 16..MAX_TOKEN_SIZE + 16) }
                val encrypted = ByteArray(size).also { input.readFully(it) }
                require(encrypted.size == size && input.read() == -1)
                val plaintext = cipher.decrypt(SealedSession(iv, encrypted), binding(endpoint, sessionId))
                try {
                    val credential = HubCredential(endpoint, plaintext.toString(Charsets.UTF_8))
                    StoredSession(sessionId, credential)
                } finally {
                    plaintext.fill(0)
                }
            }
        } catch (_: Exception) {
            // 清理与代次提升在同一锁内，坏记录绝不返回部分凭据，也不允许旧登录随后复活。
            state.generation++
            erase()
            null
        }
    }

    /** false 代表已过时代次；存储失败抛固定错误，调用者必须保持未登录。 */
    fun save(attempt: SessionAttempt, login: HubLogin): Boolean = synchronized(SecurityStorageLock) {
        if (!current(attempt)) return@synchronized false
        require(login.credential.endpoint == attempt.endpoint) { "会话地址不匹配" }
        val plaintext = login.credential.token.toByteArray(Charsets.UTF_8)
        try {
            require(plaintext.size in 1..MAX_TOKEN_SIZE)
            val sealed = cipher.encrypt(plaintext, binding(attempt.endpoint, login.session.id))
            val bytes = ByteArrayOutputStream().also { buffer ->
                DataOutputStream(buffer).use { output ->
                    output.writeInt(VERSION)
                    output.writeUTF(attempt.endpoint.bindingKey)
                    output.writeUTF(login.session.id.toString())
                    output.writeInt(sealed.iv.size)
                    output.write(sealed.iv)
                    output.writeInt(sealed.ciphertext.size)
                    output.write(sealed.ciphertext)
                }
            }.toByteArray()
            require(bytes.size <= MAX_RECORD_SIZE)
            atomicWrite(file, bytes)
            true
        } catch (_: Exception) {
            state.generation++
            erase()
            storageFailure()
        } finally {
            plaintext.fill(0)
        }
    }

    /** 注销先失效代次再删除；不触碰安装 UUID。删除失败时抛错，不能报告注销持久化成功。 */
    fun clear() = synchronized(SecurityStorageLock) {
        state.generation++
        erase()
    }

    /** 401/异步失败只可清除其所属代次，避免旧响应删除后来登录。 */
    fun clear(attempt: SessionAttempt): Boolean = synchronized(SecurityStorageLock) {
        if (!current(attempt)) return@synchronized false
        state.generation++
        erase()
        true
    }

    private fun current(attempt: SessionAttempt): Boolean =
        attempt.owner === state && attempt.generation == state.generation && attempt.endpoint == state.endpoint

    private fun erase() {
        // 即使文件删除失败也尝试删键，使残留密文不可恢复；任一失败均显式上报。
        var failed = false
        try { Files.deleteIfExists(file.toPath()) } catch (_: Exception) { failed = true }
        try { cipher.discardKey() } catch (_: Exception) { failed = true }
        if (failed) storageFailure()
    }

    companion object {
        private const val VERSION = 1
        private const val MAX_TOKEN_SIZE = 16 * 1024
        private const val MAX_RECORD_SIZE = 128 * 1024
        private val states = mutableMapOf<String, GenerationState>()

        private fun binding(endpoint: HubEndpoint, sessionId: UUID): ByteArray =
            ByteArrayOutputStream().also { buffer ->
                DataOutputStream(buffer).use {
                    it.writeInt(VERSION)
                    it.writeUTF(endpoint.bindingKey)
                    it.writeUTF(sessionId.toString())
                }
            }.toByteArray()
    }
}

/** 不使用 data class，避免凭据通过自动 toString 输出。 */
class StoredSession(val sessionId: UUID, val credential: HubCredential)

class SessionAttempt internal constructor(
    internal val owner: GenerationState,
    internal val generation: Long,
    val endpoint: HubEndpoint,
)

internal class GenerationState(var generation: Long = 0, var endpoint: HubEndpoint? = null)
