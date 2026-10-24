package top.jianxinli.relaynest.core.network

import java.net.URI
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** 规范化部署地址也是凭据绑定键；不同端口或路径前缀始终隔离。 */
class HubEndpoint private constructor(internal val url: HttpUrl) {
    val bindingKey: String get() = url.toString()

    internal fun authUrl(action: String): HttpUrl =
        requireNotNull(url.resolve("api/v1/auth/$action"))

    override fun equals(other: Any?): Boolean = other is HubEndpoint && url == other.url
    override fun hashCode(): Int = url.hashCode()
    override fun toString(): String = bindingKey

    companion object {
        fun parse(input: String): HubEndpoint {
            // 在 HttpUrl 纠正点段、反斜杠和端口前校验原始输入，避免纠正后丢失歧义证据。
            require(input.none { it.isWhitespace() || it.isISOControl() || it == '\\' }) { "Hub 地址无效" }
            val uri = try { URI(input) } catch (_: Exception) { throw IllegalArgumentException("Hub 地址无效") }
            require(uri.scheme.equals("https", ignoreCase = true) && !uri.isOpaque) { "Hub 地址必须使用 HTTPS" }
            val authority = requireNotNull(uri.rawAuthority) { "Hub 地址缺少主机" }
            require('@' !in authority && uri.rawQuery == null && uri.rawFragment == null) { "Hub 地址包含禁止字段" }
            val port = if (authority.startsWith("[")) authority.substringAfter(']', "!") else authority.substringAfter(':', "")
            if (port.isNotEmpty()) {
                val digits = if (authority.startsWith("[")) port.removePrefix(":") else port
                require((!authority.startsWith("[") || port.startsWith(":")) && digits.matches(Regex("[0-9]+")) && digits.toIntOrNull() in 1..65535) { "Hub 端口无效" }
            }
            require(!authority.endsWith(":")) { "Hub 端口无效" }
            val path = uri.rawPath.orEmpty()
            require(!Regex("%(2f|5c|25)", RegexOption.IGNORE_CASE).containsMatchIn(path)) { "Hub 路径包含编码分隔符" }
            require(path.split('/').none {
                val decoded = it.replace(Regex("%2e", RegexOption.IGNORE_CASE), ".")
                decoded == "." || decoded == ".."
            }) { "Hub 路径包含点段" }
            val parsed = requireNotNull(input.toHttpUrlOrNull()) { "Hub 地址无效" }
            val normalized = if (parsed.encodedPath.endsWith('/')) parsed else parsed.newBuilder().addPathSegment("").build()
            return HubEndpoint(normalized)
        }
    }
}
