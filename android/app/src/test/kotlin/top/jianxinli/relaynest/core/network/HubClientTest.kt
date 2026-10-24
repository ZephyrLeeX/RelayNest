package top.jianxinli.relaynest.core.network

import java.net.ServerSocket
import java.util.UUID
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.tls.HandshakeCertificates
import okhttp3.tls.HeldCertificate
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test

class HubClientTest {
    private val server = MockWebServer()
    private val target = MockWebServer()
    private lateinit var client: HubClient
    private lateinit var endpoint: HubEndpoint
    private val device = UUID.fromString("00000000-0000-4000-8000-000000000002")
    private val credential get() = HubCredential(endpoint, "fixture-token")
    private val input get() = HubLoginRequest("admin", "fixture-password", "测试设备", device)
    private val sessionJson get() = """{"session":{"id":"00000000-0000-4000-8000-000000000001","administrator":{"username":"admin"},"client_type":"android","device_id":"$device","device_name":"测试设备","idle_expires_at":"2026-10-24T01:00:00Z","absolute_expires_at":"2027-01-01T00:00:00Z"}}"""

    @Before fun start() {
        // 仅测试客户端信任临时证书；生产构造器保持平台标准 TLS。
        val cert = HeldCertificate.Builder().addSubjectAlternativeName("localhost").build()
        val serverTls = HandshakeCertificates.Builder().heldCertificate(cert).build()
        server.useHttps(serverTls.sslSocketFactory(), false)
        target.useHttps(serverTls.sslSocketFactory(), false)
        server.start()
        target.start()
        val trust = HandshakeCertificates.Builder().addTrustedCertificate(cert.certificate).build()
        client = HubClient(OkHttpClient.Builder().sslSocketFactory(trust.sslSocketFactory(), trust.trustManager).build())
        endpoint = HubEndpoint.parse(server.url("/relay/").toString())
    }
    @After fun stop() { server.shutdown(); target.shutdown() }
    private fun response(body: String = sessionJson, code: Int = 200) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
    private fun kind(result: HubResult<*>) = (result as HubResult.Failure).kind

    @Test fun exactLoginSessionLogoutContract() = runBlocking {
        val loginJson = JSONObject(sessionJson).put("access_token", "fixture-token").put("token_type", "Bearer").toString()
        server.enqueue(response(loginJson).addHeader("Set-Cookie", "relaynest_session=fixture; Path=/"))
        val login = (client.login(endpoint, input) as HubResult.Success).value
        val request = server.takeRequest()
        assertEquals("POST", request.method)
        assertEquals("/relay/api/v1/auth/login", request.path)
        assertNull(request.getHeader("Authorization"))
        val body = JSONObject(request.body.readUtf8())
        assertEquals(setOf("username", "password", "client_type", "device_name", "device_id"), body.keys().asSequence().toSet())
        assertEquals("android", body.getString("client_type"))
        assertEquals(device.toString(), body.getString("device_id"))
        assertEquals("测试设备", body.getString("device_name"))
        assertEquals("admin", body.getString("username"))
        assertEquals("fixture-password", body.getString("password"))
        assertFalse(input.toString().contains("fixture-password"))
        assertFalse(login.credential.toString().contains("fixture-token"))
        // session 无 access_token 仍须成功；服务端续期只返回元数据。
        server.enqueue(response())
        assertTrue(client.session(endpoint, login.credential) is HubResult.Success)
        val read = server.takeRequest()
        assertEquals("GET", read.method)
        assertEquals("Bearer fixture-token", read.getHeader("Authorization"))
        assertNull(read.getHeader("Cookie"))
        server.enqueue(MockResponse().setResponseCode(204))
        assertTrue(client.logout(endpoint, login.credential) is HubResult.Success)
        assertEquals("/relay/api/v1/auth/logout", server.takeRequest().path)
    }

    @Test fun redirectsNeverReplayCredentials() = runBlocking {
        val destinations = listOf(target.url("/stolen").toString(), server.url("/same-origin").toString(), "http://localhost:${target.port}/plain")
        for (location in destinations) for (code in listOf(301, 302, 303, 307, 308)) {
            server.enqueue(response("", code).addHeader("Location", location))
            assertEquals(HubError.ADDRESS, kind(client.login(endpoint, input)))
            assertEquals("/relay/api/v1/auth/login", server.takeRequest().path)
            server.enqueue(response("", code).addHeader("Location", location))
            assertEquals(HubError.ADDRESS, kind(client.session(endpoint, credential)))
            assertEquals("/relay/api/v1/auth/session", server.takeRequest().path)
            server.enqueue(response("", code).addHeader("Location", location))
            assertEquals(HubError.ADDRESS, kind(client.logout(endpoint, credential)))
            assertEquals("/relay/api/v1/auth/logout", server.takeRequest().path)
        }
        assertEquals(0, target.requestCount)
        assertEquals(45, server.requestCount)
    }

    @Test fun wrongEndpointNeverSendsBearer() = runBlocking {
        assertEquals(HubError.ADDRESS, kind(client.session(HubEndpoint.parse(target.url("/").toString()), credential)))
        assertEquals(HubError.ADDRESS, kind(client.logout(HubEndpoint.parse(server.url("/").toString()), credential)))
        assertEquals(0, server.requestCount)
        assertEquals(0, target.requestCount)
    }

    @Test fun mapsStatusWithoutRetryOrLeakingBody() = runBlocking {
        for ((status, expected) in listOf(401 to HubError.UNAUTHORIZED, 429 to HubError.RATE_LIMITED, 500 to HubError.SERVER, 503 to HubError.SERVER, 403 to HubError.HTTP)) {
            server.enqueue(response("fixture-secret", status).addHeader("Retry-After", "12"))
            val error = client.login(endpoint, input) as HubResult.Failure
            assertEquals(expected, error.kind)
            assertEquals(status, error.httpStatus)
            assertEquals(12L, error.retryAfterSeconds)
            assertFalse(error.toString().contains("fixture-secret"))
            server.takeRequest()
        }
        assertEquals(5, server.requestCount)
        server.enqueue(response("", 503))
        assertEquals(HubError.SERVER, kind(client.logout(endpoint, credential)))
        server.takeRequest()
        assertEquals(6, server.requestCount)
    }

    @Test fun zeroRetryAfterDoesNotReplayLoginOrLogout() = runBlocking {
        server.enqueue(response("", 503).addHeader("Retry-After", "0"))
        assertEquals(HubError.SERVER, kind(client.login(endpoint, input)))
        server.takeRequest()
        assertEquals(1, server.requestCount)
        server.enqueue(response("", 503).addHeader("Retry-After", "0"))
        assertEquals(HubError.SERVER, kind(client.logout(endpoint, credential)))
        server.takeRequest()
        assertEquals(2, server.requestCount)
    }

    @Test fun malformedOversizedAndMissingLoginToken() = runBlocking {
        for (body in listOf("not-json", sessionJson, JSONObject(sessionJson).put("access_token", "fixture-token").put("token_type", "Basic").toString())) {
            server.enqueue(response(body))
            assertEquals(HubError.FORMAT, kind(client.login(endpoint, input)))
        }
        server.enqueue(response(" ".repeat(262145)).setChunkedBody(" ".repeat(262145), 1024))
        assertEquals(HubError.TOO_LARGE, kind(client.session(endpoint, credential)))
        server.enqueue(response(" ".repeat(262145)))
        assertEquals(HubError.TOO_LARGE, kind(client.session(endpoint, credential)))
        server.enqueue(response().setHeader("Content-Type", "text/html"))
        assertEquals(HubError.FORMAT, kind(client.session(endpoint, credential)))
        server.enqueue(response(sessionJson.replace("\"android\"", "\"web\"")))
        assertEquals(HubError.FORMAT, kind(client.session(endpoint, credential)))
        server.enqueue(response(sessionJson.replace("00000000-0000-4000-8000-000000000002", "bad-uuid")))
        assertEquals(HubError.FORMAT, kind(client.session(endpoint, credential)))
    }

    @Test fun standardTlsRejectsUntrustedCertificate() = runBlocking {
        assertEquals(HubError.TLS, kind(HubClient().session(endpoint, credential)))
        assertEquals(0, server.requestCount)
    }

    @Test fun connectionFailureIsNetwork() = runBlocking {
        val port = ServerSocket(0).use { it.localPort }
        val missing = HubEndpoint.parse("https://localhost:$port/")
        assertEquals(HubError.NETWORK, kind(client.session(missing, HubCredential(missing, "fixture-token"))))
    }
}
