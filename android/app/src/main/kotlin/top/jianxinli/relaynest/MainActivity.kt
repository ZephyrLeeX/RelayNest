package top.jianxinli.relaynest

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import top.jianxinli.relaynest.core.ui.RelayNestTheme
import top.jianxinli.relaynest.feature.auth.AuthRepository
import top.jianxinli.relaynest.feature.auth.AuthScreen
import top.jianxinli.relaynest.feature.auth.AuthViewModel

class MainActivity : ComponentActivity() {
    private val auth: AuthViewModel by viewModels { AuthViewModel.Factory(AuthRepository(applicationContext)) }
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            RelayNestTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    AuthScreen(auth)
                }
            }
        }
    }
}
