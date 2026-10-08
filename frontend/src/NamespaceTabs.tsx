import { useLocation, useNavigate } from 'react-router-dom'
import { Tab, Tabs, TabTitleText } from '@patternfly/react-core'

// NamespaceTabs switches between a namespace's VMs and images.
export function NamespaceTabs({ ns }: { ns: string }) {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const base = `/ns/${encodeURIComponent(ns)}`
  const active = pathname.endsWith('/images') ? 'images' : 'vms'
  return (
    <Tabs activeKey={active} onSelect={(_, k) => navigate(k === 'images' ? `${base}/images` : base)} style={{ marginTop: 8 }}>
      <Tab eventKey="vms" title={<TabTitleText>Virtual machines</TabTitleText>} />
      <Tab eventKey="images" title={<TabTitleText>Images</TabTitleText>} />
    </Tabs>
  )
}
