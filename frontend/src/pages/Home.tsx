import { Card, CardBody, CardTitle, EmptyState, EmptyStateBody, Gallery, Label, PageSection, Title } from '@patternfly/react-core'
import { Link } from 'react-router-dom'
import { useSession } from '../session'

export function HomePage() {
  const { me } = useSession()
  const memberships = me?.memberships ?? []

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">Namespaces</Title>
      </PageSection>
      <PageSection>
        {memberships.length === 0 ? (
          <EmptyState titleText="No namespaces yet" headingLevel="h2">
            <EmptyStateBody>Ask an administrator for an invite to a namespace.</EmptyStateBody>
          </EmptyState>
        ) : (
          <Gallery hasGutter minWidths={{ default: '260px' }}>
            {memberships.map((m) => (
              <Card key={m.namespace}>
                <CardTitle>
                  <Link to={`/ns/${encodeURIComponent(m.namespace)}`}>{m.namespace}</Link>
                </CardTitle>
                <CardBody>
                  <Label color={m.role === 'owner' ? 'blue' : m.role === 'operator' ? 'teal' : 'grey'}>{m.role}</Label>
                </CardBody>
              </Card>
            ))}
          </Gallery>
        )}
      </PageSection>
    </>
  )
}
