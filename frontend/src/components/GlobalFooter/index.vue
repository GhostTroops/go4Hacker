<template>
  <global-footer class="footer custom-render">
    <template v-slot:links>
      <!-- <a href="https://github.com/hktalent/go4Hacker" target="_blank">Wechat</a>
      <a href="https://github.com/hktalent/go4Hacker" target="_blank">Github</a>
      <a href="https://github.com/chennqqi/" target="_blank">@chennqqi</a> -->
    </template>
    <template v-slot:copyright>
      <!-- <a href="https://www.godnslog.com" target="_blank">Copyright &copy; 2021 godnslog.com</a> -->
      <span style="font-weight:bold">{{ userInfo.username }}@{{ userInfo.company }}</span>
    </template>
  </global-footer>
</template>

<script>
import { GlobalFooter } from '@ant-design-vue/pro-layout'

export default {
  name: 'ProGlobalFooter',
  components: {
    GlobalFooter
  },
  computed: {
    userInfo () {
      return this.$store.getters.userInfo
    }
  }
}
</script>
