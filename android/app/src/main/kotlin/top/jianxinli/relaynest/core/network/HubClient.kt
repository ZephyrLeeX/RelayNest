package top.jianxinli.relaynest.core.network

import java.io.IOException
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLException
import kotlinx.coroutines.suspendCancellableCoroutine
import okhttp3.Call
import okhttp3.Callback
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okio.BufferedSink
import kotlin.coroutines.resume

sealed interface HubResult<out T> {
    data class Success<T>(val value: T) : HubResult<T>
    data class Failure(val kind: HubError, val httpStatus: Int? = null, val retryAfterSeconds: Long? = null) : HubResult<Nothing>
}

enum class HubError { UNAUTHORIZED, RATE_LIMITED, SERVER, ADDRESS, HTTP, NETWORK, TLS, FORMAT, TOO_LARGE }

/** 无持久化、无日志；只在调用者显式选定的 endpoint 上发送凭据。 */
class HubClient internal constructor(base: OkHttpClient) {
    constructor() : this(OkHttpClient())

    private val client = base.newBuilder()
        .followRedirects(false).followSslRedirects(false).retryOnConnectionFailure(false)
        .authenticator(okhttp3.Authenticator.NONE).proxyAuthenticator(okhttp3.Authenticator.NONE)
        .cookieJar(okhttp3.CookieJar.NO_COOKIES).cache(null)
        .connectTimeout(10, TimeUnit.SECONDS).readTimeout(15, TimeUnit.SECONDS)
        .writeTimeout(15, TimeUnit.SECONDS).callTimeout(30, TimeUnit.SECONDS).build()

    suspend fun login(endpoint: HubEndpoint, input: HubLoginRequest): HubResult<HubLogin> =
        execute(Request.Builder().url(endpoint.authUrl("login"))
            .post(oneShot(input.json().toRequestBody(JSON))).build()) { text ->
            val root = HubAuthDto.root(text)
            require(HubAuthDto.text(root, "token_type") == "Bearer")
            val session = HubAuthDto.session(root)
            require(session.deviceId == input.deviceId)
            HubLogin(session, HubCredential(endpoint, HubAuthDto.text(root, "access_token")))
        }

    suspend fun session(endpoint: HubEndpoint, credential: HubCredential): HubResult<HubSession> {
        if (endpoint != credential.endpoint) return HubResult.Failure(HubError.ADDRESS)
        return execute(authorized(endpoint, credential, "session").get().build()) {
            HubAuthDto.session(HubAuthDto.root(it))
        }
    }

    suspend fun logout(endpoint: HubEndpoint, credential: HubCredential): HubResult<Unit> {
        if (endpoint != credential.endpoint) return HubResult.Failure(HubError.ADDRESS)
        return execute(authorized(endpoint, credential, "logout").post(oneShot("".toRequestBody(null))).build(), 204) { Unit }
    }

    private fun authorized(endpoint: HubEndpoint, credential: HubCredential, action: String) =
        Request.Builder().url(endpoint.authUrl(action)).header("Authorization", "Bearer ${credential.token}")

    // retryOnConnectionFailure=false 仍不能禁止 503 Retry-After:0 的跟进；一次性正文阻止认证写入重放。
    private fun oneShot(body: RequestBody): RequestBody = object : RequestBody() {
        override fun contentType() = body.contentType()
        override fun contentLength() = body.contentLength()
        override fun isOneShot() = true
        override fun writeTo(sink: BufferedSink) = body.writeTo(sink)
    }

    // enqueue 在 OkHttp 线程完成 I/O 与解析；取消协程会取消请求，响应无论成功失败均关闭。
    private suspend fun <T> execute(request: Request, expected: Int = 200, decode: (String) -> T): HubResult<T> =
        suspendCancellableCoroutine { continuation ->
            val call = client.newCall(request.newBuilder().header("Accept", "application/json").build())
            continuation.invokeOnCancellation { call.cancel() }
            call.enqueue(object : Callback {
                override fun onFailure(call: Call, e: IOException) {
                    continuation.resume(HubResult.Failure(if (e is SSLException) HubError.TLS else HubError.NETWORK))
                }

                override fun onResponse(call: Call, response: Response) {
                    val result = try {
                        response.use { r ->
                            val error = when (r.code) {
                                in 300..399 -> HubError.ADDRESS
                                401 -> HubError.UNAUTHORIZED
                                429 -> HubError.RATE_LIMITED
                                in 500..599 -> HubError.SERVER
                                expected -> null
                                else -> HubError.HTTP
                            }
                            if (error != null) {
                                HubResult.Failure(error, r.code, r.header("Retry-After")?.toLongOrNull()?.takeIf { it >= 0 })
                            } else if (expected == 204) {
                                HubResult.Success(decode(""))
                            } else {
                                require(r.body?.contentType()?.let { it.type == "application" && it.subtype == "json" } == true)
                                val body = requireNotNull(r.body)
                                // 同时限制已知长度和解压后的流；不使用无界 string()/bytes()。
                                if (body.contentLength() > MAX_BODY || body.source().request(MAX_BODY + 1)) {
                                    HubResult.Failure(HubError.TOO_LARGE)
                                } else HubResult.Success(decode(body.source().readUtf8()))
                            }
                        }
                    } catch (e: IOException) {
                        HubResult.Failure(if (e is SSLException) HubError.TLS else HubError.NETWORK)
                    } catch (_: Exception) {
                        HubResult.Failure(HubError.FORMAT)
                    }
                    continuation.resume(result)
                }
            })
        }

    private companion object {
        const val MAX_BODY = 256L * 1024
        val JSON = "application/json; charset=utf-8".toMediaType()
    }
}
