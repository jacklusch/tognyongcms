<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listRoles, listPerms, createRole, updateRole, deleteRole, type RoleItem } from '../api/role'

const roles = ref<RoleItem[]>([])
const perms = ref<string[]>([])
const dialogVisible = ref(false)
const editing = ref<RoleItem | null>(null)
const form = ref({ name: '', permissions: [] as string[] })

const builtin = ['admin', 'editor', 'author']

function parsePerms(raw: string): string[] {
  try { const arr = JSON.parse(raw); return Array.isArray(arr) ? arr : [] } catch { return [] }
}

async function load() {
  const [rr, pr] = await Promise.all([listRoles(), listPerms()])
  roles.value = rr.items
  perms.value = pr.perms
}

onMounted(load)

function openCreate() {
  editing.value = null
  form.value = { name: '', permissions: [] }
  dialogVisible.value = true
}

function openEdit(r: RoleItem) {
  editing.value = r
  form.value = { name: r.name, permissions: parsePerms(r.permissions) }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name) { ElMessage.warning('请输入角色名'); return }
  try {
    if (editing.value) await updateRole(editing.value.id, form.value)
    else await createRole(form.value)
    ElMessage.success('已保存')
    dialogVisible.value = false
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '保存失败') }
}

async function remove(r: RoleItem) {
  try { await ElMessageBox.confirm(`确认删除角色 ${r.name}？`, '提示') } catch { return }
  try {
    await deleteRole(r.id)
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '删除失败') }
}
</script>

<template>
  <div>
    <h2>角色管理</h2>
    <el-button type="primary" @click="openCreate">新建角色</el-button>
    <el-table :data="roles" style="margin-top: 16px">
      <el-table-column prop="name" label="名称" width="140" />
      <el-table-column label="权限点">
        <template #default="{ row }">
          <template v-if="row.name === 'admin'"><el-tag type="danger">*</el-tag></template>
          <template v-else>
            <el-tag v-for="p in parsePerms(row.permissions)" :key="p" size="small" style="margin-right: 4px">{{ p }}</el-tag>
          </template>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button link type="primary" :disabled="builtin.includes(row.name)" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" :disabled="builtin.includes(row.name)" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑角色' : '新建角色'" width="520px">
      <el-form label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" :disabled="editing?.name === 'admin'" /></el-form-item>
        <el-form-item label="权限点">
          <el-checkbox-group v-model="form.permissions">
            <el-checkbox v-for="p in perms" :key="p" :label="p">{{ p }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
