import { useCallback, useEffect, useState } from 'react'
import { api } from './api'

// useApi GETs path and exposes the result with a reload function.
export function useApi<T>(path: string) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')

  const reload = useCallback(async () => {
    try {
      setData(await api<T>('GET', path))
      setError('')
    } catch (e) {
      setError((e as Error).message)
    }
  }, [path])

  useEffect(() => {
    reload()
  }, [reload])

  return { data, error, reload }
}
