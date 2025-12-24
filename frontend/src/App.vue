<template>
  <main class="app">
    <h1>目录打包迁移到 ZimaOS</h1>
    <p class="desc">将个人空间或团队空间目录打包成 ZIP，并根据输入的 ZimaOS 信息完成上传、解压与清理。</p>

    <form class="form" @submit.prevent="handleMigrate">
      <label class="field">
        <span>Base URL</span>
        <input v-model.trim="form.baseUrl" placeholder="http://10.147.13.1" required />
      </label>

      <label class="field">
        <span>用户名</span>
        <input v-model.trim="form.username" placeholder="请输入用户名" required />
      </label>

      <label class="field">
        <span>密码</span>
        <input v-model="form.password" type="password" placeholder="请输入密码" required />
      </label>

      <label class="field">
        <span>存储名称</span>
        <input v-model.trim="form.storage" placeholder="例如 ZimaOS-HD" required />
        <small>/media/&lt;存储名称&gt;</small>
      </label>

      <label class="field">
        <span>迁移空间</span>
        <select v-model="form.source">
          <option value="personal">个人空间 (/vol1/1000)</option>
          <option value="team">团队空间 (/vol1/@team)</option>
        </select>
      </label>

      <button class="submit" type="submit" :disabled="loading">
        {{ loading ? '正在迁移...' : '开始迁移' }}
      </button>
    </form>

    <ul class="progress">
      <li v-for="step in steps" :key="step.key" :class="['step', step.status]">
        <span class="dot"></span>
        <div class="text">
          <span class="label">{{ step.label }}</span>
          <span v-if="step.message" class="message">{{ step.message }}</span>
        </div>
      </li>
    </ul>

    <p v-if="status.message" :class="['status', status.type]">{{ status.message }}</p>

    <p class="tip">如需变更打包路径，可在部署时设置 <code>SOURCE_DIR</code> 环境变量。</p>
  </main>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'

import { MIGRATE_URL } from '@/utils/env'

const loading = ref(false)
const status = reactive({ message: '', type: 'info' as 'info' | 'error' | 'success' })

const form = reactive({
  baseUrl: '',
  username: '',
  password: '',
  storage: '',
  source: 'personal',
})

const steps = reactive([
  { key: 'login', label: '登录 ZimaOS', status: 'pending', message: '' },
  { key: 'upload', label: '打包并上传', status: 'pending', message: '' },
  { key: 'decompress', label: '解压 ZIP', status: 'pending', message: '' },
  { key: 'cleanup', label: '删除 ZIP', status: 'pending', message: '' },
])

const resetSteps = () => {
  steps.forEach((step) => {
    step.status = 'pending'
    step.message = ''
  })
}

const updateStep = (key: string, statusValue: string, message?: string) => {
  const step = steps.find((item) => item.key === key)
  if (!step) {
    return
  }

  if (statusValue === 'start') {
    step.status = 'active'
  } else if (statusValue === 'success') {
    step.status = 'success'
  } else if (statusValue === 'error') {
    step.status = 'error'
  }

  if (message) {
    step.message = message
  }
}

const handleMigrate = async () => {
  if (loading.value) {
    return
  }

  status.message = ''
  status.type = 'info'
  loading.value = true
  resetSteps()

  try {
    const response = await fetch(MIGRATE_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        baseUrl: form.baseUrl,
        username: form.username,
        password: form.password,
        storage: form.storage,
        source: form.source,
      }),
    })

    const contentType = response.headers.get('Content-Type') || ''
    const isNdjson = contentType.includes('application/x-ndjson')

    if (!response.body || !isNdjson) {
      const result = await response.json()
      if (!response.ok || result.code !== 200) {
        throw new Error(result.msg || '迁移失败')
      }

      status.type = 'success'
      status.message = `迁移完成：${result.data?.zipPath || '已上传并解压'}`
      steps.forEach((step) => (step.status = 'success'))
      return
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    const handlePayload = (payload: any) => {
      if (!payload) {
        return
      }

      if (payload.type === 'progress') {
        updateStep(payload.step, payload.status, payload.message)
      } else if (payload.type === 'done') {
        status.type = 'success'
        status.message = `迁移完成：${payload.result?.data?.zipPath || '已上传并解压'}`
      } else if (payload.type === 'error') {
        status.type = 'error'
        status.message = payload.message || '迁移失败'
      }
    }

    while (true) {
      const { value, done } = await reader.read()
      if (done) {
        break
      }

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      lines.forEach((line) => {
        const trimmed = line.trim()
        if (!trimmed) {
          return
        }
        try {
          handlePayload(JSON.parse(trimmed))
        } catch {
          return
        }
      })
    }

    if (buffer.trim()) {
      try {
        handlePayload(JSON.parse(buffer.trim()))
      } catch {
        // ignore invalid trailing data
      }
    }
  } catch (error: any) {
    status.type = 'error'
    status.message = error?.message || '迁移失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.app {
  min-height: 100vh;
  padding: 80px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  background: radial-gradient(circle at top, #f3f7ff, #ffffff 60%);
}

h1 {
  margin: 0;
  font-size: 28px;
  color: #1f2328;
}

.desc {
  margin: 0;
  font-size: 14px;
  color: #4b5563;
  text-align: center;
  max-width: 520px;
}

.form {
  width: min(520px, 90vw);
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: #ffffff;
  padding: 20px;
  border-radius: 16px;
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.08);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  color: #374151;
}

.field input {
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 14px;
}

.field select {
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 14px;
  background: #ffffff;
}

.field small {
  color: #9ca3af;
  font-size: 12px;
}

.progress {
  list-style: none;
  margin: 0;
  padding: 0;
  width: min(520px, 90vw);
  display: grid;
  gap: 8px;
}

.step {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 12px;
  background: #f8fafc;
  color: #6b7280;
}

.step .dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: currentColor;
}

.step .text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.step .label {
  font-size: 14px;
}

.step .message {
  font-size: 12px;
  color: #9ca3af;
}

.step.active {
  color: #2563eb;
  background: #eef2ff;
}

.step.success {
  color: #16a34a;
  background: #ecfdf3;
}

.step.error {
  color: #dc2626;
  background: #fef2f2;
}

.submit {
  margin-top: 8px;
  padding: 12px 32px;
  font-size: 16px;
  border: none;
  border-radius: 999px;
  cursor: pointer;
  color: #fff;
  background: linear-gradient(120deg, #2563eb, #7c3aed);
  transition: opacity 0.2s ease;
}

.submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.status {
  margin: 0;
  font-size: 14px;
}

.status.success {
  color: #16a34a;
}

.status.error {
  color: #dc2626;
}

.tip {
  margin: 0;
  font-size: 12px;
  color: #6b7280;
  text-align: center;
}

code {
  font-family: Menlo, Consolas, monospace;
  background: #f3f4f6;
  padding: 2px 4px;
  border-radius: 4px;
}
</style>
