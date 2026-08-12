<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { fetchStats, type StatsData } from '../api/stats'

const router = useRouter()
const stats = ref<StatsData | null>(null)

onMounted(async () => {
  stats.value = await fetchStats()
})
</script>

<template>
  <div>
    <h2>仪表盘</h2>
    <el-row :gutter="16" v-if="stats">
      <el-col v-for="t in stats.by_type" :key="t.type_name" :span="6">
        <el-card>
          <div class="stat-label">{{ t.type_name }}</div>
          <div class="stat-nums">
            <span>已发布 {{ t.published }}</span>
            <span>草稿 {{ t.draft }}</span>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-card v-if="stats && stats.recent.length" class="recent">
      <template #header>最近内容</template>
      <el-table :data="stats.recent" @row-click="(r: any) => router.push(`/content/${r.content.id}`)">
        <el-table-column prop="content.title" label="标题" />
        <el-table-column prop="type_name" label="类型" width="120" />
        <el-table-column prop="content.status" label="状态" width="100" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.stat-label { font-weight: 600; margin-bottom: 8px; }
.stat-nums { display: flex; gap: 16px; color: var(--el-text-color-secondary); }
.recent { margin-top: 16px; }
</style>
