import { useState, type FormEvent } from 'react'
import {
  ActionGroup,
  Alert,
  Button,
  Card,
  CardBody,
  CardTitle,
  Checkbox,
  ClipboardCopy,
  Form,
  FormGroup,
  FormSelect,
  FormSelectOption,
  Label,
  PageSection,
  TextInput,
  Title,
} from '@patternfly/react-core'
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table'
import { api, inviteStatus, roles, type Invite, type Role } from '../../api'
import { useApi } from '../../useApi'

const statusColor = { pending: 'blue', used: 'green', revoked: 'grey', expired: 'orange' } as const

export function InvitesPage() {
  const invites = useApi<Invite[]>('/admin/invites')
  const namespaces = useApi<string[]>('/admin/namespaces')

  const [email, setEmail] = useState('')
  const [namespace, setNamespace] = useState('')
  const [role, setRole] = useState<Role>('owner')
  const [makeAdmin, setMakeAdmin] = useState(false)
  const [ttlHours, setTtlHours] = useState('72')
  const [link, setLink] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function create(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    setLink('')
    try {
      const res = await api<{ url: string }>('POST', '/admin/invites', {
        email,
        namespace,
        role: namespace ? role : '',
        makeAdmin,
        ttlHours: Number(ttlHours),
      })
      setLink(res.url)
      setEmail('')
      await invites.reload()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  async function revoke(id: number) {
    try {
      await api('DELETE', `/admin/invites/${id}`)
      await invites.reload()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">Invites</Title>
      </PageSection>
      <PageSection>
        <Card>
          <CardTitle>New invite</CardTitle>
          <CardBody>
            <Form onSubmit={create} style={{ maxWidth: 560 }}>
              {(error || namespaces.error) && <Alert variant="danger" isInline title={error || namespaces.error} />}
              {link && (
                <Alert variant="success" isInline title="Invite created. Copy the link now; it is not shown again.">
                  <ClipboardCopy isReadOnly hoverTip="Copy" clickTip="Copied">
                    {link}
                  </ClipboardCopy>
                </Alert>
              )}
              <FormGroup label="Email (for your reference)" fieldId="email">
                <TextInput id="email" type="email" value={email} onChange={(_, v) => setEmail(v)} />
              </FormGroup>
              <FormGroup label="Namespace" fieldId="namespace">
                <FormSelect id="namespace" value={namespace} onChange={(_, v) => setNamespace(v)}>
                  <FormSelectOption value="" label="None (admin invite only)" />
                  {(namespaces.data ?? []).map((ns) => (
                    <FormSelectOption key={ns} value={ns} label={ns} />
                  ))}
                </FormSelect>
              </FormGroup>
              {namespace && (
                <FormGroup label="Role" fieldId="role">
                  <FormSelect id="role" value={role} onChange={(_, v) => setRole(v as Role)}>
                    {roles.map((r) => (
                      <FormSelectOption key={r} value={r} label={r} />
                    ))}
                  </FormSelect>
                </FormGroup>
              )}
              <FormGroup fieldId="admin">
                <Checkbox id="admin" label="Make this user an administrator" isChecked={makeAdmin} onChange={(_, v) => setMakeAdmin(v)} />
              </FormGroup>
              <FormGroup label="Valid for (hours, max 168)" fieldId="ttl">
                <TextInput id="ttl" type="number" min={1} max={168} value={ttlHours} onChange={(_, v) => setTtlHours(v)} />
              </FormGroup>
              <ActionGroup>
                <Button type="submit" isLoading={busy} isDisabled={busy || (!namespace && !makeAdmin)}>
                  Create invite link
                </Button>
              </ActionGroup>
            </Form>
          </CardBody>
        </Card>
      </PageSection>
      <PageSection>
        {invites.error && <Alert variant="danger" isInline title={invites.error} />}
        <Table aria-label="Invites" variant="compact">
          <Thead>
            <Tr>
              <Th>Email</Th>
              <Th>Namespace</Th>
              <Th>Role</Th>
              <Th>Admin</Th>
              <Th>Expires</Th>
              <Th>Status</Th>
              <Th screenReaderText="Actions" />
            </Tr>
          </Thead>
          <Tbody>
            {(invites.data ?? []).map((i) => {
              const status = inviteStatus(i)
              return (
                <Tr key={i.id}>
                  <Td dataLabel="Email">{i.email || '—'}</Td>
                  <Td dataLabel="Namespace">{i.namespace || '—'}</Td>
                  <Td dataLabel="Role">{i.role || '—'}</Td>
                  <Td dataLabel="Admin">{i.makeAdmin ? 'yes' : 'no'}</Td>
                  <Td dataLabel="Expires">{new Date(i.expiresAt).toLocaleString()}</Td>
                  <Td dataLabel="Status">
                    <Label color={statusColor[status]}>{status}</Label>
                  </Td>
                  <Td isActionCell>
                    {status === 'pending' && (
                      <Button variant="link" isDanger onClick={() => revoke(i.id)}>
                        Revoke
                      </Button>
                    )}
                  </Td>
                </Tr>
              )
            })}
          </Tbody>
        </Table>
      </PageSection>
    </>
  )
}
