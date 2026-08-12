<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listUsers, createUser, deleteUser, updateUserRole, updateUserPassword, type UserItem } from '../api/user'
import { listRoles, type RoleItem } from '../api/role'

const users = ref<UserItem[]>([])
const roles = ref<RoleItem[]>([])
const dialogVisible = ref(false)
const form = ref({ username: '', password: '', role_id: 0 })
const pwdDialog = ref(false)
const pwdForm = ref({ id: 0, old_password: '', password: '' })

async function load() {
  const [ur, rr] = await Promise.all([listUsers(), listRoles()])
  users.value = ur.items
  roles.value = rr.items
}

onMounted(load)

async function create() {
  if (!form.value.username || !form.value.password) { ElMessage.warning('请填写用户名和密码'); return }
  try {
    await createUser(form.value)
    ElMessage.success('已创建')
    dialogVisible.value = false
    form.value = { username: '', password: '', role_id: 0 }
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '创建失败') }
}

async function changeRole(u: UserItem, role_id: number) {
  try {
    await updateUserRole(u.id, role_id)
    ElMessage.success('已更新角色')
  } catch (e: any) { ElMessage.error(e.message ?? '操作失败'); await load() }
}

async function remove(u: UserItem) {
  try { await ElMessageBox.confirm(`确认删除用户 ${u.username}？`, '提示') } catch { return }
  try {
    await deleteUser(u.id)
    ElMessage.success('已删除')
    await load()
  } catch (e: any) { ElMessage.error(e.message ?? '删除失败') }
}

function openPwd(u: UserItem) { pwdForm.value = { id: u.id, old_password: '', password: '' }; pwdDialog.value = true }

async function savePwd() {
  try {
    await updateUserPassword(pwdForm.value.id, pwdForm.value.password, pwdForm.value.old_password || undefined)
    ElMessage.success('密码已更新')
    pwdDialog.value = false
  } catch (e: any) { ElMessage.error(e.message ?? '修改失败') }
}
</script>

<template>
  <div>
    <h2>用户管理</h2>
    <el-button type="primary" @click="dialogVisible = true">新建用户</el-button>
    <el-table :data="users" style="margin-top: 16px">
      <el-table-column prop="username" label="用户名" />
      <el-table-column label="角色" width="160">
        <template #default="{ row }">
          <el-select :model-value="row.role_id" @update:model-value="changeRole(row, $event)" style="width: 120px">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button link type="primary" @click="openPwd(row)">改密码</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="新建用户" width="420px">
      <el-form label-width="80px">
        <el-form-item label="用户名"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role_id" style="width: 100%">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="create">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="pwdDialog" title="修改密码" width="420px">
      <el-form label-width="80px">
        <el-form-item label="旧密码"><el-input v-model="pwdForm.old_password" type="password" show-password placeholder="修改他人密码可留空" /></el-form-item>
        <el-form-item label="新密码"><el-input v-model="pwdForm.password" type="password" show-password /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdDialog = false">取消</el-button>
        <el-button type="primary" @click="savePwd">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
