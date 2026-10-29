import { computed, type ComputedRef } from 'vue'
import { getActivePinia } from 'pinia'

/**
 * AI 账号所有者（role=owner）复用管理端账号页，但后端只放行自有账号的
 * 增删改查/授权/测试/用量接口；批量、导入导出、上游计费、系统设置等一律 403。
 * 账号相关组件据此隐藏对应入口并跳过请求。
 *
 * 直接读取 Pinia 中 'auth' store 的状态，而不是 import useAuthStore：
 * 后者会连带加载 api client / i18n，破坏只 mock 了 vue-i18n 的子组件单测。
 * 无活动 Pinia（单元测试直接挂载子组件）时按管理员处理，保持原有行为。
 */
export function useIsOwner(): ComputedRef<boolean> {
  const pinia = getActivePinia()
  return computed(() => {
    const authState = pinia?.state.value.auth as { user?: { role?: string } | null } | undefined
    return authState?.user?.role === 'owner'
  })
}
