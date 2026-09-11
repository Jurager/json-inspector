<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ headers: Record<string, string> }>()

interface Cookie {
  name: string
  value: string
  domain: string
  path: string
  expires: string
  flags: string
}

// Parses a single Set-Cookie header into its named parts. Multiple cookies in
// one header arrive as separate Set-Cookie headers, but the app keeps only the
// first per header name, so one row is what we can show for now.
function parseSetCookie(raw: string): Cookie {
  const parts = raw.split(';').map((s) => s.trim()).filter(Boolean)
  const first = parts[0] ?? ''
  const eq = first.indexOf('=')
  const name = eq === -1 ? first : first.slice(0, eq)
  const value = eq === -1 ? '' : first.slice(eq + 1)

  let domain = ''
  let path = '/'
  let expires = ''
  let sameSite = ''
  let httpOnly = false
  let secure = false
  for (const p of parts.slice(1)) {
    const i = p.indexOf('=')
    const key = (i === -1 ? p : p.slice(0, i)).toLowerCase()
    const val = i === -1 ? '' : p.slice(i + 1)
    if (key === 'domain') domain = val
    else if (key === 'path') path = val
    else if (key === 'expires') expires = val
    else if (key === 'samesite') sameSite = val
    else if (key === 'httponly') httpOnly = true
    else if (key === 'secure') secure = true
  }

  const flags = [httpOnly && 'HttpOnly', secure && 'Secure', sameSite && `SameSite=${sameSite}`]
    .filter(Boolean)
    .join(' · ')

  return { name, value, domain, path, expires, flags }
}

const cookies = computed<Cookie[]>(() => {
  const raw = Object.entries(props.headers).find(([k]) => k.toLowerCase() === 'set-cookie')?.[1]
  return raw ? [parseSetCookie(raw)] : []
})
</script>

<template>
  <div class="cookies">
    <table class="cookie-table">
      <thead>
        <tr>
          <th>Имя</th>
          <th>Значение</th>
          <th>Домен</th>
          <th>Путь</th>
          <th>Срок</th>
          <th>Флаги</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in cookies" :key="c.name">
          <td class="mono">{{ c.name }}</td>
          <td class="mono">{{ c.value }}</td>
          <td class="mono">{{ c.domain }}</td>
          <td class="mono">{{ c.path }}</td>
          <td class="mono">{{ c.expires }}</td>
          <td>{{ c.flags }}</td>
        </tr>
        <tr v-if="cookies.length === 0">
          <td colspan="6" class="empty-cell">Нет cookies</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
@reference "../style.css";

.cookies {
  @apply p-4 overflow-auto;
}

.cookie-table {
  @apply border-collapse w-full text-xs;
}

.cookie-table th {
  @apply text-left font-medium text-text-secondary py-1.5 px-2 border-b border-border whitespace-nowrap;
}

.cookie-table td {
  @apply text-left text-text py-1.5 px-2 border-b border-border align-top break-all;
}

.empty-cell {
  @apply text-text-tertiary text-center;
}
</style>
