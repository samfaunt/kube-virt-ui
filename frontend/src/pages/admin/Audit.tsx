import { Alert, PageSection, Title } from '@patternfly/react-core'
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table'
import type { AuditEntry } from '../../api'
import { useApi } from '../../useApi'

export function AuditPage() {
  const audit = useApi<AuditEntry[]>('/admin/audit')
  return (
    <>
      <PageSection>
        <Title headingLevel="h1">Audit log</Title>
      </PageSection>
      <PageSection>
        {audit.error && <Alert variant="danger" isInline title={audit.error} />}
        <Table aria-label="Audit log" variant="compact">
          <Thead>
            <Tr>
              <Th>Time</Th>
              <Th>Actor</Th>
              <Th>Action</Th>
              <Th>Namespace</Th>
              <Th>Target</Th>
              <Th>IP</Th>
            </Tr>
          </Thead>
          <Tbody>
            {(audit.data ?? []).map((e, i) => (
              <Tr key={i}>
                <Td dataLabel="Time">{new Date(e.at).toLocaleString()}</Td>
                <Td dataLabel="Actor">{e.actor || '—'}</Td>
                <Td dataLabel="Action">{e.action}</Td>
                <Td dataLabel="Namespace">{e.namespace || '—'}</Td>
                <Td dataLabel="Target">{e.target || '—'}</Td>
                <Td dataLabel="IP">{e.ip}</Td>
              </Tr>
            ))}
          </Tbody>
        </Table>
      </PageSection>
    </>
  )
}
