import request from '@/utils/request'

const allrecordApi = {
  dnsRecord: '/admin/allrecord/dns',
  httpRecord: '/admin/allrecord/http',
  userNameList: '/admin/allusers',
  countAllRecord: 'admin/allrecord/count'
}

export default allrecordApi

export function countAllRecord (parameter) {
  return request({
    url: allrecordApi.countAllRecord,
    method: 'get',
    params: parameter
  })
}

export function getUserNameList (parameter) {
  return request({
    url: allrecordApi.userNameList,
    method: 'get',
    params: parameter
  })
}

export function getDnsList (parameter) {
  return request({
    url: allrecordApi.dnsRecord,
    method: 'get',
    params: parameter
  })
}

export function deleteDnsList (parameter) {
  return request({
    url: allrecordApi.dnsRecord,
    method: 'delete',
    data: parameter,
    headers: {
      'Content-Type': 'application/json;charset=UTF-8'
    }
  })
}

export function getHttpList (parameter) {
  return request({
    url: allrecordApi.httpRecord,
    method: 'get',
    params: parameter
  })
}

export function deleteHttpList (parameter) {
  return request({
    url: allrecordApi.httpRecord,
    method: 'delete',
    data: parameter,
    headers: {
      'Content-Type': 'application/json;charset=UTF-8'
    }
  })
}
