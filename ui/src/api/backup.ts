import axiosInstance from '@/api/axios-instance.ts'

export const downloadBackup = async (): Promise<void> => {
  const response = await axiosInstance.get('/backup/download', {
    responseType: 'blob',
  })

  const blob = new Blob([response.data], { type: 'application/octet-stream' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = `mwp-backup-${new Date().toISOString().split('T')[0]}.db`

  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

export const sendInstantBackup = async (): Promise<void> => {
  await axiosInstance.post('/backup/send-now')
}

export interface RestoreStagedResult {
  staged_path: string
  message: string
}

export const uploadRestoreBackup = async (
  file: File
): Promise<RestoreStagedResult> => {
  const formData = new FormData()
  formData.append('backup', file)

  const { data } = await axiosInstance.post('/backup/restore', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })

  return data.data
}
