package top.jianxinli.relaynest.core.security

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.File
import java.time.Instant
import java.util.UUID
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import top.jianxinli.relaynest.core.network.HubCredential
import top.jianxinli.relaynest.core.network.HubEndpoint
import top.jianxinli.relaynest.core.network.HubLogin
import top.jianxinli.relaynest.core.network.HubSession

class SessionStorageTest {
    @get:Rule val temporary = TemporaryFolder()
    private val endpoint = HubEndpoint.parse("https://hub.example:8443/base/")
    private val other = HubEndpoint.parse("https://hub.example:8443/other/")
    private val token = "test-bearer-only"
    private val keys = JvmKeys()
    private val cipher = SessionCipher(keys)
    private val directory get() = temporary.root
    private val record get() = File(directory, "hub-session.v1")
    private fun store() = SessionStore(directory, cipher)
    private fun login(address: HubEndpoint = endpoint, id: UUID = UUID.randomUUID()): HubLogin = HubLogin(
        HubSession(id, "admin", UUID.randomUUID(), "test-device", Instant.MAX, Instant.MAX),
        HubCredential(address, token),
    )

    @Test fun identityPersistsAcrossInstancesAndSessionClear() {
        val id = InstallationIdentity(directory).getOrCreate()
        val store = store()
        val attempt = store.begin(endpoint)
        val login = login()
        assertNotEquals(id, login.session.id)
        assertTrue(store.save(attempt, login))
        store.clear()
        assertEquals(id, InstallationIdentity(directory).getOrCreate())
        assertFalse(record.exists())
        assertNull(keys.key)
    }

    @Test fun differentInstallDirectoriesGenerateDifferentIdentity() {
        assertNotEquals(InstallationIdentity(temporary.newFolder()).getOrCreate(), InstallationIdentity(temporary.newFolder()).getOrCreate())
    }

    @Test fun corruptedIdentityFailsWithoutReplacement() {
        File(directory, "installation-id").writeText("broken")
        assertThrows(IllegalStateException::class.java) { InstallationIdentity(directory).getOrCreate() }
        assertEquals("broken", File(directory, "installation-id").readText())
    }

    @Test fun gcmRoundTripRandomIvAndBindingAuthentication() {
        val plaintext = token.toByteArray()
        val binding = "binding".toByteArray()
        val first = cipher.encrypt(plaintext, binding)
        val second = cipher.encrypt(plaintext, binding)
        assertEquals(12, first.iv.size)
        assertFalse(first.iv.contentEquals(second.iv))
        assertArrayEquals(plaintext, cipher.decrypt(first, binding))
        assertThrows(Exception::class.java) { cipher.decrypt(first, "other".toByteArray()) }
        first.ciphertext[0] = (first.ciphertext[0].toInt() xor 1).toByte()
        assertThrows(Exception::class.java) { cipher.decrypt(first, binding) }
    }

    @Test fun restoreAcrossInstancesUsesNormalizedAddressAndMinimalEncryptedRecord() {
        val first = store()
        val result = login()
        assertTrue(first.save(first.begin(endpoint), result))
        val text = record.readBytes().toString(Charsets.ISO_8859_1)
        assertFalse(text.contains(token))
        assertFalse(text.contains("admin"))
        assertFalse(text.contains("test-device"))
        val second = store()
        val restored = second.restore(second.begin(HubEndpoint.parse("https://HUB.example:8443/base")))!!
        assertEquals(result.session.id, restored.sessionId)
        assertEquals(token, restored.credential.token)
        assertEquals(endpoint, restored.credential.endpoint)
    }

    @Test fun addressSwitchErasesRecordAndRejectsOldTicketEvenWhenSwitchingBack() {
        val store = store()
        val old = store.begin(endpoint)
        store.save(old, login())
        val changed = store.begin(other)
        assertNull(store.restore(changed))
        assertFalse(record.exists())
        store.begin(endpoint)
        assertFalse(store.save(old, login()))
    }

    @Test fun staleLoginAndStale401CannotResurrectOrDeleteNewSession() {
        val store = store()
        val old = store.begin(endpoint)
        store.clear()
        assertFalse(store.save(old, login()))
        val current = store.begin(endpoint)
        val result = login()
        assertTrue(store.save(current, result))
        assertFalse(store.clear(old))
        assertEquals(result.session.id, store.restore(current)!!.sessionId)
    }

    @Test fun generationIsSharedBetweenStoreInstances() {
        val first = store()
        val old = first.begin(endpoint)
        store().clear()
        assertFalse(first.isCurrent(old))
        assertFalse(first.save(old, login()))
    }

    @Test fun retryInvalidatesPreviousLoginAndForeignCredentialsAreRejected() {
        val store = store()
        val old = store.begin(endpoint)
        val retry = store.begin(endpoint)
        assertFalse(store.save(old, login()))
        assertThrows(IllegalArgumentException::class.java) { store.save(retry, login(other)) }
        assertFalse(record.exists())
        assertTrue(store.save(retry, login()))
    }

    @Test fun clearIsIdempotentAndRestorationRemainsEmpty() {
        val store = store()
        store.save(store.begin(endpoint), login())
        store.clear()
        store.clear()
        assertNull(store.restore(store.begin(endpoint)))
        assertFalse(record.exists())
    }

    @Test fun sessionMetadataTamperingFailsAuthenticationAndCleans() {
        val store = store()
        val attempt = store.begin(endpoint)
        store.save(attempt, login())
        rewriteMetadata { address, _ -> address to UUID.randomUUID().toString() }
        assertNull(store.restore(attempt))
        assertFalse(record.exists())
        assertNull(keys.key)
    }

    @Test fun endpointMetadataTamperingCannotRebindCiphertext() {
        val store = store()
        store.save(store.begin(endpoint), login())
        rewriteMetadata { _, id -> other.bindingKey to id }
        // 使用另一目录模拟重启后首次选择新地址，避免只证明内存地址比较。
        val copiedDirectory = temporary.newFolder()
        record.copyTo(File(copiedDirectory, record.name))
        val copied = SessionStore(copiedDirectory, cipher)
        assertNull(copied.restore(copied.begin(other)))
        assertFalse(File(copiedDirectory, record.name).exists())
    }

    @Test fun corruptTruncatedAndTrailingRecordsCleanAndPermitFreshLogin() {
        val store = store()
        for (transform in listOf<(ByteArray) -> ByteArray>(
            { it.copyOf(8) },
            { it + byteArrayOf(1) },
            { bytes -> bytes.also { it[it.lastIndex] = (it.last().toInt() xor 1).toByte() } },
        )) {
            val attempt = store.begin(endpoint)
            store.save(attempt, login())
            record.writeBytes(transform(record.readBytes()))
            assertNull(store.restore(attempt))
            assertFalse(record.exists())
            assertNull(keys.key)
            assertFalse(store.save(attempt, login()))
            val fresh = store.begin(endpoint)
            assertTrue(store.save(fresh, login()))
            assertNotNull(store.restore(fresh))
        }
    }

    @Test fun missingAndInvalidatedKeyCleanWithoutPlaintextFallback() {
        val store = store()
        val attempt = store.begin(endpoint)
        store.save(attempt, login())
        keys.key = null
        assertNull(store.restore(attempt))
        assertFalse(record.exists())
        val fresh = store.begin(endpoint)
        store.save(fresh, login())
        keys.failDecrypt = true
        assertNull(store.restore(fresh))
        assertFalse(record.exists())
        assertNull(keys.key)
    }

    @Test fun failedWriteInvalidatesAttemptAndCleansKey() {
        val store = store()
        val attempt = store.begin(endpoint)
        record.mkdir()
        File(record, "occupied").writeText("test")
        assertThrows(IllegalStateException::class.java) { store.save(attempt, login()) }
        assertFalse(store.isCurrent(attempt))
        assertNull(keys.key)
    }

    @Test fun logoutWaitsForInFlightSaveThenRemovesIt() {
        val entered = CountDownLatch(1)
        val release = CountDownLatch(1)
        keys.beforeEncrypt = { entered.countDown(); check(release.await(5, TimeUnit.SECONDS)) }
        val store = store()
        val attempt = store.begin(endpoint)
        val pool = Executors.newFixedThreadPool(2)
        try {
            val saving = pool.submit<Boolean> { store.save(attempt, login()) }
            assertTrue(entered.await(5, TimeUnit.SECONDS))
            val clearing = pool.submit { store.clear() }
            release.countDown()
            assertTrue(saving.get(5, TimeUnit.SECONDS))
            clearing.get(5, TimeUnit.SECONDS)
            assertFalse(record.exists())
            assertFalse(store.isCurrent(attempt))
        } finally {
            release.countDown()
            pool.shutdownNow()
        }
    }

    @Test fun replacementNeverExposesPartialRecordToConcurrentReader() {
        val store = store()
        val attempt = store.begin(endpoint)
        store.save(attempt, login())
        val pool = Executors.newSingleThreadExecutor()
        try {
            val reading = pool.submit {
                repeat(100) {
                    DataInputStream(ByteArrayInputStream(record.readBytes())).use { input ->
                        assertEquals(1, input.readInt())
                        assertEquals(endpoint.bindingKey, input.readUTF())
                        UUID.fromString(input.readUTF())
                        assertEquals(12, input.readInt())
                        input.readFully(ByteArray(12))
                        val size = input.readInt()
                        assertTrue(size >= 16)
                        input.readFully(ByteArray(size))
                        assertEquals(-1, input.read())
                    }
                }
            }
            repeat(30) { assertTrue(store.save(attempt, login())) }
            reading.get(5, TimeUnit.SECONDS)
            assertEquals(listOf(record.name), directory.list()!!.toList())
        } finally { pool.shutdownNow() }
    }

    private fun rewriteMetadata(change: (String, String) -> Pair<String, String>) {
        val input = DataInputStream(ByteArrayInputStream(record.readBytes()))
        val version = input.readInt()
        val metadata = change(input.readUTF(), input.readUTF())
        val remainder = input.readBytes()
        val buffer = ByteArrayOutputStream()
        DataOutputStream(buffer).use {
            it.writeInt(version)
            it.writeUTF(metadata.first)
            it.writeUTF(metadata.second)
            it.write(remainder)
        }
        record.writeBytes(buffer.toByteArray())
    }

    // JVM 真实执行 GCM；密钥来源仅用于测试，不代表 AndroidKeyStore 或硬件保障。
    private class JvmKeys : SessionKeySource {
        var key: SecretKey? = null
        var failDecrypt = false
        var beforeEncrypt: (() -> Unit)? = null
        override fun encryptionKey(): SecretKey {
            beforeEncrypt?.invoke()
            return key ?: KeyGenerator.getInstance("AES").generateKey().also { key = it }
        }
        override fun decryptionKey(): SecretKey {
            check(!failDecrypt)
            return checkNotNull(key)
        }
        override fun delete() { key = null }
    }
}
