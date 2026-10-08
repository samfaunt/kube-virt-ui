import { api, ApiError, vmPath, type Image } from './api'

export interface UploadMeta {
  name: string
  type: 'iso' | 'disk'
  size: string
  storageClass?: string
}

export type UploadStage = { stage: 'creating' } | { stage: 'waiting' } | { stage: 'uploading'; percent: number }

const GiB = 1024 ** 3

// suggestImage derives a name, type and disk size from the chosen file.
// Disk sizes leave room for filesystem overhead; qcow2 images may expand,
// so they get at least 10 GiB.
export function suggestImage(file: File): UploadMeta {
  const base = file.name.replace(/\.[^.]+$/, '').toLowerCase()
  const name = base.replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 63) || 'image'
  const type = /\.iso$/i.test(file.name) ? 'iso' : 'disk'
  const gib = Math.ceil((file.size * 1.1) / GiB) + 1
  return { name, type, size: `${type === 'disk' ? Math.max(10, gib) : gib}Gi` }
}

async function waitUntilReady(ns: string, name: string) {
  const deadline = Date.now() + 5 * 60_000
  for (;;) {
    const img = (await api<Image[]>('GET', vmPath(ns) + '/images')).find((i) => i.name === name)
    if (img?.problem) throw new Error(img.problem)
    if (img?.phase === 'UploadReady') return
    if (Date.now() > deadline)
      throw new Error(`Timed out waiting for the image to be ready for upload (stuck in ${img?.phase || 'Pending'}). Delete it and try again.`)
    await new Promise((r) => setTimeout(r, 2000))
  }
}

// XMLHttpRequest rather than fetch: fetch cannot report upload progress.
function send(ns: string, name: string, file: File, onProgress: (percent: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `/api${vmPath(ns)}/images/${encodeURIComponent(name)}/upload`)
    xhr.setRequestHeader('X-Requested-With', 'kvui')
    xhr.setRequestHeader('Content-Type', 'application/octet-stream')
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress(Math.floor((e.loaded / e.total) * 100))
    xhr.onload = () => {
      if (xhr.status === 204) return resolve()
      let msg = xhr.statusText
      try {
        msg = JSON.parse(xhr.responseText).error ?? msg
      } catch {
        /* not JSON, e.g. a proxy's error page */
      }
      reject(new ApiError(xhr.status, msg))
    }
    xhr.onerror = () => reject(new Error('Upload failed: network error'))
    xhr.send(file)
  })
}

// uploadImage creates the image, waits for CDI's upload server and streams
// the file. CDI then converts the data; the image list shows that phase.
export async function uploadImage(ns: string, file: File, meta: UploadMeta, onStage: (s: UploadStage) => void) {
  onStage({ stage: 'creating' })
  await api('POST', vmPath(ns) + '/images', meta)
  onStage({ stage: 'waiting' })
  await waitUntilReady(ns, meta.name)
  onStage({ stage: 'uploading', percent: 0 })
  await send(ns, meta.name, file, (percent) => onStage({ stage: 'uploading', percent }))
}
