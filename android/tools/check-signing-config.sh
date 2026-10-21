#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
test_dir=$(mktemp -d)
trap 'chmod u+r "$test_dir/unreadable" 2>/dev/null || :; rm -rf "$test_dir"' EXIT
touch "$test_dir/empty" "$test_dir/unreadable"
chmod 000 "$test_dir/unreadable"

# 所有输入仅来自本脚本创建的临时文件和非秘密标记，不继承用户签名配置。
names=(RELAYNEST_ANDROID_KEYSTORE_PATH RELAYNEST_ANDROID_KEYSTORE_PASSWORD RELAYNEST_ANDROID_KEY_ALIAS RELAYNEST_ANDROID_KEY_PASSWORD)
unset_args=()
for name in "${names[@]}"; do unset_args+=(-u "$name"); done
marker="test-only-$(basename "$test_dir")"
gradle_args=(--offline --console=plain --no-configuration-cache)

run_check() {
    local mode=$1 case_name=$2 expected=$3 reason=$4
    local -a inputs=() tasks=()
    case "$case_name" in
        missing) ;;
        partial) inputs=("${names[1]}=$marker-password") ;;
        *)
            local reference="$test_dir/$case_name"
            [[ "$case_name" != directory ]] || reference="$test_dir"
            inputs=("${names[0]}=$reference" "${names[1]}=$marker-password" "${names[2]}=$marker-alias" "${names[3]}=$marker-key-password")
            if [[ "$case_name" == blank ]]; then
                inputs[0]="${names[0]}=$test_dir/empty"
                inputs[3]="${names[3]}= "
            fi
            ;;
    esac
    if [[ "$mode" == request ]]; then
        tasks=(:app:assembleRelease -PrelaynestReleaseSigning=true)
    elif [[ "$mode" == enabled-diagnostic ]]; then
        tasks=(:app:checkReleaseSigningConfig -PrelaynestReleaseSigning=true)
    else
        tasks=(:app:checkReleaseSigningConfig)
    fi
    local status=0
    env "${unset_args[@]}" "${inputs[@]}" ./gradlew "${tasks[@]}" "${gradle_args[@]}" >"$test_dir/log" 2>&1 || status=$?
    # 只报告用例名，不回显 Gradle 原始日志、输入、路径或别名。
    if rg -q -F -e "$marker" -e "$test_dir" "$test_dir/log"; then
        echo "FAIL $mode/$case_name: 日志泄漏测试输入"
        exit 1
    fi
    if [[ "$expected" == failure && "$status" == 0 ]] || [[ "$expected" == success && "$status" != 0 ]] || ! rg -q -F "$reason" "$test_dir/log"; then
        echo "FAIL $mode/$case_name: 退出码或固定诊断不符合预期"
        exit 1
    fi
    if [[ "$mode" == request ]] && rg -q '^> Task ' "$test_dir/log"; then
        echo "FAIL $mode/$case_name: 无效配置进入任务执行"
        exit 1
    fi
    if [[ "$case_name" == missing ]]; then
        for name in "${names[@]}"; do
            if ! rg -q -F "$name" "$test_dir/log"; then
                echo "FAIL $mode/$case_name: 未列全缺失字段"
                exit 1
            fi
        done
    fi
    echo "PASS $mode/$case_name"
}

for mode in request diagnostic; do
    run_check "$mode" missing failure '正式签名配置缺少字段'
    run_check "$mode" partial failure '正式签名配置缺少字段'
    run_check "$mode" blank failure 'RELAYNEST_ANDROID_KEY_PASSWORD'
    run_check "$mode" nonexistent failure '正式签名文件引用无效'
    run_check "$mode" directory failure '正式签名文件引用无效'
    if [[ ! -r "$test_dir/unreadable" ]]; then
        run_check "$mode" unreadable failure '正式签名文件引用无效'
    else
        echo "NOT_RUN $mode/unreadable: 当前用户可绕过文件权限"
    fi
done
run_check diagnostic empty success '未验证口令、别名或签名'
run_check enabled-diagnostic empty success '未验证口令、别名或签名'

status=0
env "${unset_args[@]}" ./gradlew :app:assembleRelease -PrelaynestReleaseSigning=true --configuration-cache --offline --console=plain >"$test_dir/log" 2>&1 || status=$?
if [[ "$status" == 0 ]] || ! rg -q -F '正式签名请求必须使用 --no-configuration-cache' "$test_dir/log"; then
    echo 'FAIL request/configuration-cache'
    exit 1
fi
echo 'PASS request/configuration-cache'
