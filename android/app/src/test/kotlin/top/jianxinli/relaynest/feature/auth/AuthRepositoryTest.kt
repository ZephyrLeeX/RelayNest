package top.jianxinli.relaynest.feature.auth

import java.io.File
import java.nio.file.Files
import java.time.Instant
import java.util.UUID
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.coroutines.asCoroutineDispatcher
import org.junit.Assert.*
import org.junit.Test
import top.jianxinli.relaynest.core.network.*
import top.jianxinli.relaynest.core.security.*

class AuthRepositoryTest {
    private val endpoint = HubEndpoint.parse("https://hub.example/relay/")
    private val second = HubEndpoint.parse("https://second.example/")
    private val device = UUID.randomUUID()
    private fun login(endpoint: HubEndpoint = this.endpoint) = HubLogin(
        HubSession(UUID.randomUUID(), "admin", device, "phone", Instant.parse("2030-01-01T00:00:00Z"), Instant.parse("2030-02-01T00:00:00Z")),
        HubCredential(endpoint, "fixture-token"),
    )

    @Test fun `login saves server session and stable installation id without retaining password in state`() = runBlocking {
        val f = fixture()
        f.repo.connect(endpoint.bindingKey)
        val authenticated = login()
        f.remote.loginResult = HubResult.Success(authenticated)
        f.repo.login("admin", "transient-secret", "phone")
        assertEquals(AuthPhase.AUTHENTICATED, f.repo.state.value.phase)
        assertEquals(authenticated.session.id, f.repo.state.value.session?.id)
        assertEquals(device, f.remote.loginDevice)
        assertNotEquals(device, f.repo.state.value.session?.id)
        assertEquals(authenticated.session.id, f.storage.stored?.sessionId)
        assertFalse(f.repo.state.value.toString().contains("transient-secret"))
        assertFalse(f.repo.state.value.toString().contains("fixture-token"))
    }

    @Test fun `login failures never authenticate or reuse password automatically`() = runBlocking {
        for (error in listOf(HubError.UNAUTHORIZED, HubError.NETWORK, HubError.RATE_LIMITED, HubError.SERVER, HubError.TLS, HubError.FORMAT)) {
            val f = fixture()
            f.repo.connect(endpoint.bindingKey)
            f.remote.loginResult = HubResult.Failure(error, retryAfterSeconds = 12)
            f.repo.login("admin", "secret", "phone")
            assertNull(f.repo.state.value.session)
            assertNull(f.storage.stored)
            assertEquals(1, f.remote.loginCalls)
            if (error in listOf(HubError.NETWORK, HubError.RATE_LIMITED, HubError.SERVER)) {
                assertEquals(AuthPhase.RETRYABLE_ERROR, f.repo.state.value.phase)
                f.repo.retryRestore()
                assertEquals(1, f.remote.loginCalls)
                assertEquals(AuthPhase.RETRYABLE_ERROR, f.repo.state.value.phase)
            }
        }
    }

    @Test fun `startup remains restoring until actual session GET succeeds`() = runBlocking {
        val f = fixture()
        val saved = login()
        f.storage.seed(saved)
        val response = CompletableDeferred<HubResult<HubSession>>()
        f.remote.sessionAction = { response.await() }
        val restore = launch(start = CoroutineStart.UNDISPATCHED) { f.repo.initialize() }
        assertEquals(AuthPhase.RESTORING, f.repo.state.value.phase)
        assertNull(f.repo.state.value.session)
        assertEquals(1, f.remote.sessionCalls)
        assertSame(saved.credential, f.remote.sessionCredential)
        response.complete(HubResult.Success(saved.session))
        restore.join()
        assertEquals(AuthPhase.AUTHENTICATED, f.repo.state.value.phase)
        assertEquals(saved.session.id, f.repo.state.value.session?.id)
    }

    @Test fun `startup 401 expires and deletes only local session keeping installation`() = runBlocking {
        val f = fixture()
        f.storage.seed(login())
        f.remote.sessionResult = HubResult.Failure(HubError.UNAUTHORIZED, 401)
        f.repo.initialize()
        assertEquals(AuthPhase.EXPIRED, f.repo.state.value.phase)
        assertNull(f.storage.stored)
        assertEquals(device, f.storage.deviceId())
        assertTrue(f.repo.state.value.message.contains("重新登录"))
    }

    @Test fun `startup transient errors keep credential for explicit GET retry`() = runBlocking {
        for (error in listOf(HubError.NETWORK, HubError.RATE_LIMITED, HubError.SERVER)) {
            val f = fixture()
            val saved = login()
            f.storage.seed(saved)
            f.remote.sessionResult = HubResult.Failure(error)
            f.repo.initialize()
            assertEquals(AuthPhase.RETRYABLE_ERROR, f.repo.state.value.phase)
            assertNull(f.repo.state.value.session)
            assertEquals(saved.session.id, f.storage.stored?.sessionId)
            f.remote.sessionResult = HubResult.Success(saved.session)
            f.repo.retryRestore()
            assertEquals(AuthPhase.AUTHENTICATED, f.repo.state.value.phase)
            assertEquals(0, f.remote.loginCalls)
        }
    }

    @Test fun `mismatched restored session or installation never authenticates`() = runBlocking {
        for (wrongDevice in listOf(false, true)) {
            val f = fixture()
            val saved = login()
            f.storage.seed(saved)
            f.remote.sessionResult = HubResult.Success(if (wrongDevice) saved.session.copy(deviceId = UUID.randomUUID()) else saved.session.copy(id = UUID.randomUUID()))
            f.repo.initialize()
            assertEquals(AuthPhase.EXPIRED, f.repo.state.value.phase)
            assertNull(f.storage.stored)
        }
    }

    @Test fun `offline logout ends local session and explicitly leaves remote unconfirmed`() = runBlocking {
        val f = fixture()
        val saved = authenticate(f)
        f.remote.logoutResult = HubResult.Failure(HubError.NETWORK)
        f.repo.logout()
        assertEquals(1, f.remote.logoutCalls)
        assertSame(saved.credential, f.remote.logoutCredential)
        assertEquals(AuthPhase.UNAUTHENTICATED, f.repo.state.value.phase)
        assertNull(f.repo.state.value.session)
        assertNull(f.storage.stored)
        assertTrue(f.repo.state.value.message.contains("未确认"))
        f.repo.retryRestore()
        assertEquals(AuthPhase.UNAUTHENTICATED, f.repo.state.value.phase)
    }

    @Test fun `logout 429 and 5xx do not claim revocation success`() = runBlocking {
        for (error in listOf(HubError.RATE_LIMITED, HubError.SERVER)) {
            val f = fixture()
            authenticate(f)
            f.remote.logoutResult = HubResult.Failure(error)
            f.repo.logout()
            assertNull(f.storage.stored)
            assertTrue(f.repo.state.value.message.contains("未确认"))
        }
    }

    @Test fun `successful logout or already revoked session confirm ended remote session`() = runBlocking {
        for (result in listOf(HubResult.Success(Unit), HubResult.Failure(HubError.UNAUTHORIZED))) {
            val f = fixture()
            authenticate(f)
            f.remote.logoutResult = result
            f.repo.logout()
            assertTrue(f.repo.state.value.message.contains("服务端会话已失效"))
            assertNull(f.storage.stored)
        }
    }

    @Test fun `late login after address change cannot persist or publish old identity`() = runBlocking {
        val f = fixture()
        f.repo.connect(endpoint.bindingKey)
        val response = CompletableDeferred<HubResult<HubLogin>>()
        f.remote.loginAction = { withContext(NonCancellable) { response.await() } }
        val old = launch(start = CoroutineStart.UNDISPATCHED) { f.repo.login("admin", "secret", "phone") }
        assertEquals(AuthPhase.LOGGING_IN, f.repo.state.value.phase)
        f.repo.changeAddress()
        assertNull(f.storage.stored)
        assertNull(f.storage.savedEndpoint)
        f.repo.connect(second.bindingKey)
        val newer = login(second)
        f.remote.loginAction = null
        f.remote.loginResult = HubResult.Success(newer)
        f.repo.login("admin", "new-secret", "phone")
        response.complete(HubResult.Success(login()))
        old.join()
        assertEquals(second, f.repo.state.value.endpoint)
        assertEquals(newer.session.id, f.repo.state.value.session?.id)
        assertEquals(newer.session.id, f.storage.stored?.sessionId)
    }

    @Test fun `late restoring 401 cannot delete a newer session`() = runBlocking {
        val f = fixture()
        f.storage.seed(login())
        val response = CompletableDeferred<HubResult<HubSession>>()
        f.remote.sessionAction = { withContext(NonCancellable) { response.await() } }
        val old = launch(start = CoroutineStart.UNDISPATCHED) { f.repo.initialize() }
        f.repo.changeAddress()
        f.repo.connect(endpoint.bindingKey)
        val newer = authenticate(f)
        response.complete(HubResult.Failure(HubError.UNAUTHORIZED))
        old.join()
        assertEquals(newer.session.id, f.storage.stored?.sessionId)
        assertEquals(newer.session.id, f.repo.state.value.session?.id)
    }

    @Test fun `logout invalidates pending login and late logout cannot overwrite a new login`() = runBlocking {
        val f = fixture()
        f.repo.connect(endpoint.bindingKey)
        val loginResponse = CompletableDeferred<HubResult<HubLogin>>()
        f.remote.loginAction = { withContext(NonCancellable) { loginResponse.await() } }
        val pending = launch(start = CoroutineStart.UNDISPATCHED) { f.repo.login("admin", "secret", "phone") }
        f.repo.logout()
        loginResponse.complete(HubResult.Success(login()))
        pending.join()
        assertNull(f.storage.stored)
        assertEquals(AuthPhase.UNAUTHENTICATED, f.repo.state.value.phase)

        f.remote.loginAction = null
        authenticate(f)
        val logoutResponse = CompletableDeferred<HubResult<Unit>>()
        f.remote.logoutAction = { logoutResponse.await() }
        val logout = launch(start = CoroutineStart.UNDISPATCHED) { f.repo.logout() }
        assertNull(f.storage.stored)
        assertNull(f.repo.state.value.session)
        val newer = authenticate(f)
        logoutResponse.complete(HubResult.Success(Unit))
        logout.join()
        assertEquals(newer.session.id, f.repo.state.value.session?.id)
        assertEquals(newer.session.id, f.storage.stored?.sessionId)
    }

    @Test fun `storage failures never display authenticated or confirmed cleanup`() = runBlocking {
        val f = fixture()
        f.repo.connect(endpoint.bindingKey)
        f.storage.failSave = true
        f.remote.loginResult = HubResult.Success(login())
        f.repo.login("admin", "secret", "phone")
        assertEquals(AuthPhase.ERROR, f.repo.state.value.phase)
        assertNull(f.repo.state.value.session)
        f.storage.failSave = false
        authenticate(f)
        f.storage.failClear = true
        f.repo.logout()
        assertEquals(AuthPhase.ERROR, f.repo.state.value.phase)
        assertTrue(f.repo.state.value.message.contains("清理未确认"))
        assertEquals(1, f.remote.logoutCalls)
        f.repo.retryRestore()
        assertEquals(AuthPhase.ERROR, f.repo.state.value.phase)
    }

    @Test fun `address configuration stores only canonical endpoint atomically and removes malformed values`() {
        val dir = Files.createTempDirectory("auth-address-test").toFile()
        try {
            val file = File(dir, "hub-address.v1")
            val config = HubAddressFile(file)
            assertNull(config.read())
            config.write(HubEndpoint.parse("https://HUB.example:443/relay"))
            assertEquals(endpoint, config.read())
            assertEquals(endpoint.bindingKey, file.readText())
            assertEquals(listOf("hub-address.v1"), dir.list()!!.toList())
            for (bad in listOf("https://user:secret@hub.example/", "broken", "a".repeat(8193))) {
                file.writeText(bad)
                assertNull(config.read())
                assertFalse(file.exists())
            }
            config.write(second)
            assertEquals(second, config.read())
            config.write(null)
            assertFalse(file.exists())
        } finally { dir.deleteRecursively() }
    }

    @Test fun `disk work stays off caller thread and address change isolates before deletion finishes`() = runBlocking {
        val caller = Thread.currentThread()
        val store = FakeStorage(device)
        val threads = mutableSetOf<Thread>()
        var blocked = false
        val entered = CompletableDeferred<Unit>()
        val allow = CountDownLatch(1)
        val storage = object : AuthStorage by store {
            override fun clear() {
                threads.add(Thread.currentThread())
                assertNotSame(caller, Thread.currentThread())
                if (blocked) {
                    entered.complete(Unit)
                    check(allow.await(5, TimeUnit.SECONDS))
                }
                store.clear()
            }
        }
        Executors.newSingleThreadExecutor().asCoroutineDispatcher().use { io ->
            val remote = FakeRemote()
            val repo = AuthRepository(storage, remote, io)
            val f = Fixture(store, remote, repo)
            authenticate(f)
            blocked = true
            val changing = launch(start = CoroutineStart.UNDISPATCHED) { repo.changeAddress() }
            try {
                entered.await()
                assertEquals(AuthPhase.UNAUTHENTICATED, repo.state.value.phase)
                assertNull(repo.state.value.session)
                assertNull(repo.state.value.endpoint)
            } finally { allow.countDown() }
            changing.join()
            assertNull(store.stored)
            assertNull(store.savedEndpoint)
        }
        assertEquals(1, threads.size)
    }

    private fun fixture(): Fixture {
        val storage = FakeStorage(device)
        val remote = FakeRemote()
        return Fixture(storage, remote, AuthRepository(storage, remote, Dispatchers.Unconfined))
    }
    private suspend fun authenticate(f: Fixture): HubLogin {
        if (f.repo.state.value.endpoint == null) f.repo.connect(endpoint.bindingKey)
        val saved = login(f.repo.state.value.endpoint!!)
        f.remote.loginResult = HubResult.Success(saved)
        f.repo.login("admin", "secret", "phone")
        return saved
    }
    private class Fixture(val storage: FakeStorage, val remote: FakeRemote, val repo: AuthRepository)

    private class FakeStorage(private val device: UUID) : AuthStorage {
        private val state = GenerationState()
        var savedEndpoint: HubEndpoint? = null
        var stored: StoredSession? = null
        var failSave = false
        var failClear = false
        fun seed(login: HubLogin) { savedEndpoint = login.credential.endpoint; stored = StoredSession(login.session.id, login.credential) }
        override fun endpoint() = savedEndpoint
        override fun remember(endpoint: HubEndpoint?) { savedEndpoint = endpoint }
        override fun begin(endpoint: HubEndpoint): SessionAttempt {
            state.generation++
            state.endpoint = endpoint
            return SessionAttempt(state, state.generation, endpoint)
        }
        override fun restore(attempt: SessionAttempt) = if (isCurrent(attempt)) stored else null
        override fun save(attempt: SessionAttempt, login: HubLogin): Boolean {
            check(!failSave)
            if (!isCurrent(attempt)) return false
            stored = StoredSession(login.session.id, login.credential)
            return true
        }
        override fun isCurrent(attempt: SessionAttempt) = attempt.owner === state && attempt.generation == state.generation && attempt.endpoint == state.endpoint
        override fun clear() { state.generation++; check(!failClear); stored = null }
        override fun clear(attempt: SessionAttempt): Boolean {
            if (!isCurrent(attempt)) return false
            clear()
            return true
        }
        override fun deviceId() = device
    }

    private class FakeRemote : AuthRemote {
        var loginResult: HubResult<HubLogin> = HubResult.Failure(HubError.NETWORK)
        var sessionResult: HubResult<HubSession> = HubResult.Failure(HubError.NETWORK)
        var logoutResult: HubResult<Unit> = HubResult.Success(Unit)
        var loginAction: (suspend () -> HubResult<HubLogin>)? = null
        var sessionAction: (suspend () -> HubResult<HubSession>)? = null
        var logoutAction: (suspend () -> HubResult<Unit>)? = null
        var loginCalls = 0
        var sessionCalls = 0
        var logoutCalls = 0
        var loginDevice: UUID? = null
        var sessionCredential: HubCredential? = null
        var logoutCredential: HubCredential? = null
        override suspend fun login(endpoint: HubEndpoint, request: HubLoginRequest): HubResult<HubLogin> {
            loginCalls++
            loginDevice = request.deviceId
            return loginAction?.invoke() ?: loginResult
        }
        override suspend fun session(endpoint: HubEndpoint, credential: HubCredential): HubResult<HubSession> {
            sessionCalls++
            sessionCredential = credential
            return sessionAction?.invoke() ?: sessionResult
        }
        override suspend fun logout(endpoint: HubEndpoint, credential: HubCredential): HubResult<Unit> {
            logoutCalls++
            logoutCredential = credential
            return logoutAction?.invoke() ?: logoutResult
        }
    }
}
