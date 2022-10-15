<template>
  <div>
<!--    <div class="table-page-search-wrapper">
      <a-form layout="inline">
        <a-row :gutter="48">
          <a-col :md="8" :sm="24">
            <a-form-item :label="$t('Username')">
              <a-input v-model="queryParam.username" placeholder=""/>
            </a-form-item>
          </a-col>
          <a-col :md="8" :sm="24">
            <a-form-item :label="$t('Email')">
              <a-input v-model="queryParam.email" placeholder=""/>
            </a-form-item>
          </a-col>
          <a-col :md="8" :sm="24">
            <a-form-item :label="$t('UpdateTime')">
              <a-date-picker v-model="queryParam.date" style="width: 100%" placeholder="请输入更新日期"/>
            </a-form-item>
          </a-col>
          <a-col :md="!advanced && 8 || 24" :sm="24">
            <span class="table-page-search-submitButtons" :style="advanced && { float: 'right', overflow: 'hidden' } || {} ">
              <a-button type="primary" @click="$refs.table.refresh(true)">{{ $t('Query') }}</a-button>
              <a-button style="margin-left: 8px" @click="() => queryParam = {}">{{ $t('Reset') }}</a-button>
            </span>
          </a-col>
        </a-row>
      </a-form>
    </div> -->

    <div class="table-operator">
      <!-- <a-button style="margin-left: 8px" @click="handleSelect" v-if="selectedRowKeys.length > 0">
          {{ $t('Count Select') }}
      </a-button> -->
      <!-- <a-button type="dashed" @click="tableOption">{{ optionAlertShow && $t('Close') || $t('Open') }} {{ $t('Batch') }}</a-button> -->
    </div>

    <s-table
      ref="table"
      size="default"
      rowKey="id"
      :columns="columns"
      :data="loadData"
      :alert="options.alert"
      :rowSelection="options.rowSelection"
    >
      <span slot="serial" slot-scope="text, record, index">
        {{ index + 1 }}
      </span>
    </s-table>
  </div>
</template>

<script>
import moment from 'moment'
import { STable } from '@/components'
import { countAllRecord } from '@/api/allrecord'

export default {
  name: 'TableList',
  components: {
    STable
  },
  data () {
    return {
      mdl: {},
      // 高级搜索 展开/关闭
      advanced: false,
      // 查询参数
      queryParam: {},
      // 表头
      columns: [
        {
          title: '#',
          scopedSlots: { customRender: 'serial' },
          dataIndex: 'id'
        },
        {
          title: this.$t('Company'),
          dataIndex: 'company'
        },
        {
          title: this.$t('Username'),
          dataIndex: 'username'
        },
        {
          title: this.$t('FullName'),
          dataIndex: 'full_name'
        },
        {
          title: this.$t('ShortId'),
          dataIndex: 'short_id'
        },
        {
          title: this.$t('Email'),
          dataIndex: 'email'
        },
        {
          title: this.$t('Role'),
          dataIndex: 'role',
          customRender: (text, record, index) => {
            if (this.$i18n.locale === 'zh-CN') {
              return text.name
            } else {
              return text.id
            }
          }
        },
        {
          title: this.$t('DNS'),
          dataIndex: 'dns_count'
        },
        {
          title: this.$t('HTTP'),
          dataIndex: 'http_count'
        }
      ],
      // 加载数据方法 必须为 Promise 对象
      loadData: parameter => {
        console.log('loadData.parameter', parameter)
        return countAllRecord(Object.assign(parameter, this.queryParam))
          .then(res => {
            console.log('loadData', res.result)
            return res.result
          })
      },
      selectedRowKeys: [],
      selectedRows: [],

      // custom table alert & rowSelection
      options: {
        alert: { show: true, clear: () => { this.selectedRowKeys = [] } },
        rowSelection: {
          selectedRowKeys: this.selectedRowKeys,
          onChange: this.onSelectChange
        }
      },
      optionAlertShow: true
    }
  },
  created () {
    this.tableOption()
    // getRoleList({ t: new Date() })
  },
  methods: {
    tableOption () {
      if (!this.optionAlertShow) {
        this.options = {
          alert: { show: true, clear: () => { this.selectedRowKeys = [] } },
          rowSelection: {
            selectedRowKeys: this.selectedRowKeys,
            onChange: this.onSelectChange
          }
        }
        this.optionAlertShow = true
      } else {
        this.options = {
          alert: false,
          rowSelection: null
        }
        this.optionAlertShow = false
      }
    },
    handleSelect () {
      console.log('handleSelect')
    },
    activated () {
      console.log('LIST activated')
      this.$refs.table.refresh() // refresh() 不传参默认值 false 不刷新到分页第一页
    },
    onSelectChange (selectedRowKeys, selectedRows) {
      this.selectedRowKeys = selectedRowKeys
      this.selectedRows = selectedRows
    },
    resetSearchForm () {
      this.queryParam = {
        date: moment(new Date())
      }
    }
  }
}
</script>
