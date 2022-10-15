<script src="//unpkg.com/vue@2/dist/vue.js"></script>
<script src="//unpkg.com/element-ui@2.15.8/lib/index.js"></script>
<div id="app">
<el-tabs type="border-card" shadow="always">
  <el-tab-pane label="User">User</el-tab-pane>
  <el-tab-pane label="Config">Config</el-tab-pane>
  <el-tab-pane label="Role">Role</el-tab-pane>
  <el-tab-pane label="Task">Task</el-tab-pane>
</el-tabs>
</div>

@import url("//unpkg.com/element-ui@2.15.8/lib/theme-chalk/index.css");

new Vue().$mount('#app')