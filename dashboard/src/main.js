import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia } from 'pinia'
import App from './App.vue'
import Dashboard from './views/Dashboard.vue'
import LoginNew from './views/LoginNew.vue'
import ProtectedEvents from './views/ProtectedEvents.vue'
import Integrations from './views/Integrations.vue'
import Account from './views/Account.vue'
import Documentation from './views/Documentation.vue'
import './style.css'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: Dashboard, meta: { requiresAuth: true } },
  { path: '/protected-events', component: ProtectedEvents, meta: { requiresAuth: true } },
  { path: '/integrations', component: Integrations, meta: { requiresAuth: true } },
  { path: '/account', component: Account, meta: { requiresAuth: true } },
  { path: '/documentation', component: Documentation, meta: { requiresAuth: true } },
  { path: '/login', component: LoginNew },
  { path: '/signup', component: LoginNew, props: { mode: 'signup' } }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// Auth guard
router.beforeEach((to, from, next) => {
  const isAuthenticated = localStorage.getItem('auth_token')

  // Clear any modal states when navigating to login
  if (to.path === '/login' || to.path === '/signup') {
    // Reset any global modal/overlay states
    document.body.style.overflow = 'auto'
    // Remove any fixed overlays that might be lingering
    const overlays = document.querySelectorAll('.fixed.inset-0')
    overlays.forEach(overlay => overlay.remove())
  }

  if (to.meta.requiresAuth && !isAuthenticated) {
    next('/login')
  } else if ((to.path === '/login' || to.path === '/signup') && isAuthenticated) {
    next('/dashboard')
  } else {
    next()
  }
})

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
