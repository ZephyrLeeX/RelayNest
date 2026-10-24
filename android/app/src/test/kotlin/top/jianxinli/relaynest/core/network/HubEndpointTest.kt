package top.jianxinli.relaynest.core.network

import org.junit.Assert.*
import org.junit.Test

class HubEndpointTest {
    @Test fun normalizationAndPrefix() {
        assertEquals(HubEndpoint.parse("https://EXAMPLE.test"), HubEndpoint.parse("HTTPS://example.test:443/"))
        assertNotEquals(HubEndpoint.parse("https://example.test"), HubEndpoint.parse("https://example.test:8443"))
        val prefixed = HubEndpoint.parse("https://example.test:8443/relay")
        assertEquals("https://example.test:8443/relay/api/v1/auth/login", prefixed.authUrl("login").toString())
        assertNotEquals(prefixed, HubEndpoint.parse("https://example.test:8443/"))
        assertEquals("https://[::1]:8443/", HubEndpoint.parse("https://[::1]:8443").bindingKey)
        assertEquals(HubEndpoint.parse("https://bücher.test"), HubEndpoint.parse("https://xn--bcher-kva.test/"))
    }

    @Test fun rejectsAmbiguousOrUnsafeInput() {
        listOf(
            "http://example.test", "https://user@example.test", "https://@example.test", "https://example.test?",
            "https://example.test#", "https://example.test:0", "https://example.test:65536", "https://example.test:",
            "https://example.test:-1", "https://example.test:x", "https://example.test/a/../b",
            "https://example.test/./", "https://example.test/%2E%2e/", "https://example.test/.%2e/",
            "https://example.test/a%2fb", "https://example.test/a%5Cb", "https://example.test/%252f",
            "https://example.test/a\\b", " https://example.test", "https://example.test/\n", "https://example.test/%",
        ).forEach { input ->
            assertThrows(input, IllegalArgumentException::class.java) { HubEndpoint.parse(input) }
        }
    }
}
