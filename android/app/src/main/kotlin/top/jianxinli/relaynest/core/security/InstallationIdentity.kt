package top.jianxinli.relaynest.core.security

import android.content.Context
import java.io.File
import java.io.FileOutputStream
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.util.UUID

/** 安装身份独立于登录会话；只在应用私有、不备份的目录生成。需在 IO 线程调用。 */
class InstallationIdentity internal constructor(private val directory: File) {
    constructor(context: Context) : this(context.noBackupFilesDir)

    fun getOrCreate(): UUID = synchronized(SecurityStorageLock) {
        val file = File(directory, "installation-id")
        if (file.exists()) {
            // 损坏时失败关闭，不能默默变更设备身份；错误不包含文件内容。
            if (file.length() != 36L) storageFailure()
            val value = try { file.readText(Charsets.UTF_8) } catch (_: Exception) { storageFailure() }
            if (value.length != 36) storageFailure()
            val id = try { UUID.fromString(value) } catch (_: Exception) { storageFailure() }
            if (id.toString() != value) storageFailure()
            id
        } else {
            val id = UUID.randomUUID()
            try { atomicWrite(file, id.toString().toByteArray(Charsets.UTF_8)) } catch (_: Exception) { storageFailure() }
            id
        }
    }
}

// 所有同进程存储实例串行化；当前应用没有跨进程组件，不声称提供跨进程锁。
internal object SecurityStorageLock

internal fun storageFailure(): Nothing = throw IllegalStateException("安全存储不可用，请重新登录")

/** 同目录临时文件写入并 fsync 后原子替换；读者永远不会看到半条记录。 */
internal fun atomicWrite(file: File, bytes: ByteArray) {
    val directory = requireNotNull(file.parentFile)
    check(directory.isDirectory || directory.mkdirs())
    val temporary = File.createTempFile(".security-", ".tmp", directory)
    try {
        FileOutputStream(temporary).use { stream ->
            stream.write(bytes)
            stream.fd.sync()
        }
        Files.move(temporary.toPath(), file.toPath(), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
    } finally {
        temporary.delete()
    }
}
