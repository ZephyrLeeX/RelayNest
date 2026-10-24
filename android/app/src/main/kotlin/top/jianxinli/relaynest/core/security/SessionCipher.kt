package top.jianxinli.relaynest.core.security

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** 测试只替换密钥来源，AES/GCM 封装与生产一致；不代表设备 Keystore 验证。 */
class SessionCipher internal constructor(private val keys: SessionKeySource) {
    constructor() : this(AndroidSessionKeySource())

    internal fun encrypt(plaintext: ByteArray, binding: ByteArray): SealedSession {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        // 不传 IV，让提供者生成随机 IV，避免重复使用 GCM nonce。
        cipher.init(Cipher.ENCRYPT_MODE, keys.encryptionKey())
        cipher.updateAAD(binding)
        return SealedSession(cipher.iv, cipher.doFinal(plaintext))
    }

    internal fun decrypt(sealed: SealedSession, binding: ByteArray): ByteArray {
        require(sealed.iv.size == 12 && sealed.ciphertext.size >= 16)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, keys.decryptionKey(), GCMParameterSpec(128, sealed.iv))
        cipher.updateAAD(binding)
        return cipher.doFinal(sealed.ciphertext)
    }

    internal fun discardKey() = keys.delete()
}

internal class SealedSession(val iv: ByteArray, val ciphertext: ByteArray)

internal interface SessionKeySource {
    fun encryptionKey(): SecretKey
    fun decryptionKey(): SecretKey
    fun delete()
}

private class AndroidSessionKeySource : SessionKeySource {
    private val alias = "relaynest.hub.session.v1"

    private fun store(): KeyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }

    override fun encryptionKey(): SecretKey {
        val existing = store().getKey(alias, null)
        if (existing != null) return existing as SecretKey
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").apply {
            init(
                KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setRandomizedEncryptionRequired(true)
                    .build(),
            )
        }.generateKey()
    }

    // 恢复时绝不生成替代密钥；缺失或失效必须丢弃旧会话。
    override fun decryptionKey(): SecretKey = store().getKey(alias, null) as? SecretKey ?: error("会话密钥不可用")

    override fun delete() = store().deleteEntry(alias)
}
