package top.jianxinli.relaynest.feature.auth

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

class AuthViewModel(private val repository: AuthRepository) : ViewModel() {
    val state = repository.state
    private var operation: Job? = null

    init { run { repository.initialize() } }

    // UNDISPATCHED 保证点击即提升 Repository 代次，不等下一帧才隔离旧身份。
    private fun run(action: suspend () -> Unit) {
        operation?.cancel()
        operation = viewModelScope.launch(start = CoroutineStart.UNDISPATCHED) { action() }
    }

    fun connect(address: String) = run { repository.connect(address) }
    fun login(username: String, password: String, deviceName: String) = run { repository.login(username, password, deviceName) }
    fun retryRestore() = run { repository.retryRestore() }
    fun changeAddress() = run { repository.changeAddress() }
    fun logout() = run { repository.logout() }

    class Factory(private val repository: AuthRepository) : ViewModelProvider.Factory {
        @Suppress("UNCHECKED_CAST")
        override fun <T : ViewModel> create(modelClass: Class<T>): T {
            require(modelClass.isAssignableFrom(AuthViewModel::class.java))
            return AuthViewModel(repository) as T
        }
    }
}
