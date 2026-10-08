import { useState } from 'react'
import { Alert, Button, Flex, FlexItem, FormSelect, FormSelectOption, Label, PageSection, Title } from '@patternfly/react-core'
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table'
import { api, roles, type Role, type UserWithMemberships } from '../../api'
import { useSession } from '../../session'
import { useApi } from '../../useApi'

function AddMembership({ user, namespaces, onDone }: { user: UserWithMemberships; namespaces: string[]; onDone: (err?: string) => void }) {
  const available = namespaces.filter((ns) => !user.memberships.some((m) => m.namespace === ns))
  const [namespace, setNamespace] = useState('')
  const [role, setRole] = useState<Role>('viewer')
  if (available.length === 0) return null
  return (
    <Flex spaceItems={{ default: 'spaceItemsSm' }} flexWrap={{ default: 'nowrap' }}>
      <FlexItem>
        <FormSelect aria-label="Namespace" value={namespace} onChange={(_, v) => setNamespace(v)}>
          <FormSelectOption value="" label="Add namespace…" />
          {available.map((ns) => (
            <FormSelectOption key={ns} value={ns} label={ns} />
          ))}
        </FormSelect>
      </FlexItem>
      <FlexItem>
        <FormSelect aria-label="Role" value={role} onChange={(_, v) => setRole(v as Role)}>
          {roles.map((r) => (
            <FormSelectOption key={r} value={r} label={r} />
          ))}
        </FormSelect>
      </FlexItem>
      <FlexItem>
        <Button
          variant="secondary"
          isDisabled={!namespace}
          onClick={() =>
            api('PUT', `/admin/users/${user.id}/memberships/${namespace}`, { role }).then(
              () => {
                setNamespace('')
                onDone()
              },
              (e) => onDone(e.message),
            )
          }
        >
          Add
        </Button>
      </FlexItem>
    </Flex>
  )
}

export function UsersPage() {
  const { me } = useSession()
  const users = useApi<UserWithMemberships[]>('/admin/users')
  const namespaces = useApi<string[]>('/admin/namespaces')
  const [error, setError] = useState('')

  async function act(fn: () => Promise<unknown>) {
    setError('')
    try {
      await fn()
    } catch (e) {
      setError((e as Error).message)
    }
    await users.reload()
  }

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">Users</Title>
      </PageSection>
      <PageSection>
        {(error || users.error) && <Alert variant="danger" isInline title={error || users.error} />}
        <Table aria-label="Users" variant="compact">
          <Thead>
            <Tr>
              <Th>Username</Th>
              <Th>Email</Th>
              <Th>Namespaces</Th>
              <Th>Status</Th>
              <Th screenReaderText="Actions" />
            </Tr>
          </Thead>
          <Tbody>
            {(users.data ?? []).map((u) => (
              <Tr key={u.id}>
                <Td dataLabel="Username">
                  {u.username} {u.isAdmin && <Label color="purple">admin</Label>}
                </Td>
                <Td dataLabel="Email">{u.email || '—'}</Td>
                <Td dataLabel="Namespaces">
                  <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsSm' }}>
                    <Flex spaceItems={{ default: 'spaceItemsXs' }}>
                      {u.memberships.map((m) => (
                        <Label
                          key={m.namespace}
                          onClose={() => {
                            if (confirm(`Remove ${u.username} from ${m.namespace}? Their access is revoked immediately.`))
                              act(() => api('DELETE', `/admin/users/${u.id}/memberships/${m.namespace}`))
                          }}
                        >
                          {m.namespace}: {m.role}
                        </Label>
                      ))}
                    </Flex>
                    <AddMembership user={u} namespaces={namespaces.data ?? []} onDone={(err) => act(async () => { if (err) throw new Error(err) })} />
                  </Flex>
                </Td>
                <Td dataLabel="Status">{u.disabled ? <Label color="red">disabled</Label> : <Label color="green">active</Label>}</Td>
                <Td isActionCell>
                  {u.id !== me?.user.id && (
                    <Button variant="link" isDanger={!u.disabled} onClick={() => act(() => api('PATCH', `/admin/users/${u.id}`, { disabled: !u.disabled }))}>
                      {u.disabled ? 'Enable' : 'Disable'}
                    </Button>
                  )}
                </Td>
              </Tr>
            ))}
          </Tbody>
        </Table>
      </PageSection>
    </>
  )
}
