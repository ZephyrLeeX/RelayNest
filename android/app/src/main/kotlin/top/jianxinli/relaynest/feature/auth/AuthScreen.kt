package top.jianxinli.relaynest.feature.auth

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle

@Composable
fun AuthScreen(viewModel: AuthViewModel) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var address by remember { mutableStateOf("") }
    var username by remember { mutableStateOf("") }
    var deviceName by remember { mutableStateOf("Android") }
    // 密码不经过 saved state/VM/磁盘；提交、离开前台及退出组合时清理引用。
    var password by remember { mutableStateOf("") }
    val owner = LocalLifecycleOwner.current
    DisposableEffect(owner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_STOP) password = ""
            if (event == Lifecycle.Event.ON_START && viewModel.state.value.phase == AuthPhase.AUTHENTICATED) viewModel.retryRestore()
        }
        owner.lifecycle.addObserver(observer)
        onDispose { password = ""; owner.lifecycle.removeObserver(observer) }
    }
    LaunchedEffect(state.phase, state.endpoint) { password = "" }

    val busy = state.phase == AuthPhase.LOGGING_IN || state.phase == AuthPhase.RESTORING
    Column(Modifier.fillMaxSize().safeDrawingPadding().imePadding().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Text("RelayNest", style = MaterialTheme.typography.headlineLarge)
        Text("Bot Hub 登录", style = MaterialTheme.typography.titleLarge)
        Text(state.message)
        state.endpoint?.let { Text(it.bindingKey) }
        if (busy) LinearProgressIndicator(Modifier.fillMaxWidth())
        if (state.endpoint == null && !busy) {
            OutlinedTextField(address, { address = it }, label = { Text("Hub HTTPS 地址") }, singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri), modifier = Modifier.fillMaxWidth())
            Button(onClick = { viewModel.connect(address) }, enabled = address.isNotBlank()) { Text("连接 Hub") }
        } else if (state.phase == AuthPhase.AUTHENTICATED) {
            Text("账号：${state.session?.administratorUsername}")
            Text("设备：${state.session?.deviceName}")
            Button(onClick = viewModel::retryRestore) { Text("验证会话") }
            Button(onClick = viewModel::logout) { Text("注销") }
        } else if (!busy && state.endpoint != null) {
            OutlinedTextField(username, { username = it }, label = { Text("账号") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(password, { password = it }, label = { Text("密码") }, singleLine = true,
                visualTransformation = PasswordVisualTransformation(), keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                modifier = Modifier.fillMaxWidth())
            OutlinedTextField(deviceName, { deviceName = it }, label = { Text("设备名称") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Button(onClick = {
                val submitted = password
                password = ""
                viewModel.login(username, submitted, deviceName)
            }, enabled = username.isNotBlank() && password.isNotEmpty() && deviceName.isNotBlank()) { Text("登录") }
            if (state.canRetryRestore) {
                TextButton(onClick = viewModel::retryRestore) { Text("重试恢复（不重发密码）") }
            }
        }
        if (state.endpoint != null || busy) {
            TextButton(onClick = { password = ""; address = ""; viewModel.changeAddress() }) { Text("更换地址 / 取消") }
        }
    }
}
